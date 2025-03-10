/*

Copyright (c) 2021 - Present. Blend Labs, Inc. All rights reserved
Use of this source code is governed by a MIT license that can be found in the LICENSE file.

*/

package semver

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// The compiled regular expression used to test the validity of a version.
var versionRegexp *regexp.Regexp

// VersionRegexpRaw is the raw regular expression string used for
// testing the validity of a version.
const VersionRegexpRaw string = `v?([0-9]+(\.[0-9]+)*?)` +
	`(-?([0-9A-Za-z\-~]+(\.[0-9A-Za-z\-~]+)*))?` +
	`(\+([0-9A-Za-z\-~]+(\.[0-9A-Za-z\-~]+)*))?` +
	`?`

// Version represents a single version.
type Version struct {
	metadata string
	pre      string
	segments []int64
	si       int
}

func init() {
	versionRegexp = regexp.MustCompile("^" + VersionRegexpRaw + "$")
}

// NewVersion parses the given version and returns a new Version.
func NewVersion(v string) (*Version, error) {
	matches := versionRegexp.FindStringSubmatch(v)
	if matches == nil {
		return nil, fmt.Errorf("malformed version: %s", v)
	}
	segmentsStr := strings.Split(matches[1], ".")
	segments := make([]int64, len(segmentsStr))
	si := 0
	for i, str := range segmentsStr {
		val, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return nil, fmt.Errorf(
				"error parsing version: %s", err)
		}

		segments[i] = int64(val)
		si++
	}

	for i := len(segments); i < 3; i++ {
		segments = append(segments, 0)
	}

	return &Version{
		metadata: matches[7],
		pre:      matches[4],
		segments: segments,
		si:       si,
	}, nil
}

// Must is a helper that wraps a call to a function returning (*Version, error)
// and panics if error is non-nil.
func Must(v *Version, err error) *Version {
	if err != nil {
		panic(err)
	}

	return v
}

// Compare compares this version to another version. This
// returns -1, 0, or 1 if this version is smaller, equal,
// or larger than the other version, respectively.
//
// If you want boolean results, use the LessThan, Equal,
// or GreaterThan methods.
func (v *Version) Compare(other *Version) int {
	// A quick, efficient equality check
	if v.String() == other.String() {
		return 0
	}

	segmentsSelf := v.Segments64()
	segmentsOther := other.Segments64()

	// If the segments are the same, we must compare on prerelease info
	if reflect.DeepEqual(segmentsSelf, segmentsOther) {
		preSelf := v.Prerelease()
		preOther := other.Prerelease()
		if preSelf == "" && preOther == "" {
			return 0
		}
		if preSelf == "" {
			return 1
		}
		if preOther == "" {
			return -1
		}

		return comparePrereleases(preSelf, preOther)
	}

	// Get the highest specificity (hS), or if they're equal, just use segmentSelf length
	lenSelf := len(segmentsSelf)
	lenOther := len(segmentsOther)
	hS := lenSelf
	if lenSelf < lenOther {
		hS = lenOther
	}
	// Compare the segments
	// Because a constraint could have more/less specificity than the version it's
	// checking, we need to account for a lopsided or jagged comparison
	for i := 0; i < hS; i++ {
		if i > lenSelf-1 {
			// This means Self had the lower specificity
			// Check to see if the remaining segments in Other are all zeros
			if !allZero(segmentsOther[i:]) {
				// if not, it means that Other has to be greater than Self
				return -1
			}
			break
		} else if i > lenOther-1 {
			// this means Other had the lower specificity
			// Check to see if the remaining segments in Self are all zeros -
			if !allZero(segmentsSelf[i:]) {
				//if not, it means that Self has to be greater than Other
				return 1
			}
			break
		}
		lhs := segmentsSelf[i]
		rhs := segmentsOther[i]
		if lhs == rhs {
			continue
		} else if lhs < rhs {
			return -1
		}
		// Otherwis, rhs was > lhs, they're not equal
		return 1
	}

	// if we got this far, they're equal
	return 0
}

func allZero(segs []int64) bool {
	for _, s := range segs {
		if s != 0 {
			return false
		}
	}
	return true
}

func comparePart(preSelf string, preOther string) int {
	if preSelf == preOther {
		return 0
	}

	var selfInt int64
	selfNumeric := true
	selfInt, err := strconv.ParseInt(preSelf, 10, 64)
	if err != nil {
		selfNumeric = false
	}

	var otherInt int64
	otherNumeric := true
	otherInt, err = strconv.ParseInt(preOther, 10, 64)
	if err != nil {
		otherNumeric = false
	}

	// if a part is empty, we use the other to decide
	if preSelf == "" {
		if otherNumeric {
			return -1
		}
		return 1
	}

	if preOther == "" {
		if selfNumeric {
			return 1
		}
		return -1
	}

	if selfNumeric && !otherNumeric {
		return -1
	} else if !selfNumeric && otherNumeric {
		return 1
	} else if !selfNumeric && !otherNumeric && preSelf > preOther {
		return 1
	} else if selfInt > otherInt {
		return 1
	}

	return -1
}

func comparePrereleases(v string, other string) int {
	// the same pre release!
	if v == other {
		return 0
	}

	// split both pre releases for analyze their parts
	selfPreReleaseMeta := strings.Split(v, ".")
	otherPreReleaseMeta := strings.Split(other, ".")

	selfPreReleaseLen := len(selfPreReleaseMeta)
	otherPreReleaseLen := len(otherPreReleaseMeta)

	biggestLen := otherPreReleaseLen
	if selfPreReleaseLen > otherPreReleaseLen {
		biggestLen = selfPreReleaseLen
	}

	// loop for parts to find the first difference
	for i := 0; i < biggestLen; i = i + 1 {
		partSelfPre := ""
		if i < selfPreReleaseLen {
			partSelfPre = selfPreReleaseMeta[i]
		}

		partOtherPre := ""
		if i < otherPreReleaseLen {
			partOtherPre = otherPreReleaseMeta[i]
		}

		compare := comparePart(partSelfPre, partOtherPre)
		// if parts are equals, continue the loop
		if compare != 0 {
			return compare
		}
	}

	return 0
}

// Equal tests if two versions are equal.
func (v *Version) Equal(o *Version) bool {
	return v.Compare(o) == 0
}

// GreaterThan tests if this version is greater than another version.
func (v *Version) GreaterThan(o *Version) bool {
	return v.Compare(o) > 0
}

// LessThan tests if this version is less than another version.
func (v *Version) LessThan(o *Version) bool {
	return v.Compare(o) < 0
}

// Metadata returns any metadata that was part of the version
// string.
//
// Metadata is anything that comes after the "+" in the version.
// For example, with "1.2.3+beta", the metadata is "beta".
func (v *Version) Metadata() string {
	return v.metadata
}

// Prerelease returns any prerelease data that is part of the version,
// or blank if there is no prerelease data.
//
// Prerelease information is anything that comes after the "-" in the
// version (but before any metadata). For example, with "1.2.3-beta",
// the prerelease information is "beta".
func (v *Version) Prerelease() string {
	return v.pre
}

// Segments returns the numeric segments of the version as a slice of ints.
//
// This excludes any metadata or pre-release information. For example,
// for a version "1.2.3-beta", segments will return a slice of
// 1, 2, 3.
func (v *Version) Segments() []int {
	segmentSlice := make([]int, len(v.segments))
	for i, v := range v.segments {
		segmentSlice[i] = int(v)
	}
	return segmentSlice
}

// Segments64 returns the numeric segments of the version as a slice of int64s.
//
// This excludes any metadata or pre-release information. For example,
// for a version "1.2.3-beta", segments will return a slice of
// 1, 2, 3.
func (v *Version) Segments64() []int64 {
	return v.segments
}

// String returns the full version string included pre-release
// and metadata information.
func (v *Version) String() string {
	var buf bytes.Buffer
	fmtParts := make([]string, len(v.segments))
	for i, s := range v.segments {
		// We can ignore err here since we've pre-parsed the values in segments
		str := strconv.FormatInt(s, 10)
		fmtParts[i] = str
	}
	fmt.Fprint(&buf, strings.Join(fmtParts, "."))
	if v.pre != "" {
		fmt.Fprintf(&buf, "-%s", v.pre)
	}
	if v.metadata != "" {
		fmt.Fprintf(&buf, "+%s", v.metadata)
	}

	return buf.String()
}

// Major returns the Major segment, or the highest order segment.
func (v *Version) Major() (major int64) {
	if len(v.segments) < 1 {
		return
	}
	major = v.segments[0]
	return
}

// Minor returns the Minor segment, or the second highest order segment.
func (v *Version) Minor() (minor int64) {
	if len(v.segments) < 2 {
		return
	}
	minor = v.segments[1]
	return
}

// Patch returns the Patch segment, or the third highest order segment.
func (v *Version) Patch() (patch int64) {
	if len(v.segments) < 3 {
		return
	}
	patch = v.segments[2]
	return
}

// BumpMajor increments the Major field by 1 and resets all other fields to their default values
func (v *Version) BumpMajor() {
	v.segments = []int64{v.Major() + 1, 0, 0}
	v.pre = ""
	v.metadata = ""
}

// BumpMinor increments the Minor field by 1 and resets all other fields to their default values
func (v *Version) BumpMinor() {
	v.segments = []int64{v.Major(), v.Minor() + 1, 0}
	v.pre = ""
	v.metadata = ""
}

// BumpPatch increments the Patch field by 1 and resets all other fields to their default values
func (v *Version) BumpPatch() {
	v.segments = []int64{v.Major(), v.Minor(), v.Patch() + 1}
	v.pre = ""
	v.metadata = ""
}

// Collection is a type that implements the sort.Interface interface
// so that versions can be sorted.
type Collection []*Version

func (v Collection) Len() int {
	return len(v)
}

func (v Collection) Less(i, j int) bool {
	return v[i].LessThan(v[j])
}

func (v Collection) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}
