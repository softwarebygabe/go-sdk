/*

Copyright (c) 2021 - Present. Blend Labs, Inc. All rights reserved
Use of this source code is governed by a MIT license that can be found in the LICENSE file.

*/

// Copyright 2013 Julien Schmidt. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package web

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

/*
//nolint:unused
/*
func printChildren(n *RouteNode, prefix string) {
	fmt.Printf(" %02d:%02d %s%s[%d] %v %t %d \r\n", n.Priority, n.MaxParams, prefix, n.Path, len(n.Children), n.Route, n.IsWildcard, n.RouteNodeType)
	for l := len(n.Path); l > 0; l-- {
		prefix += " "
	}
	for _, child := range n.Children {
		printChildren(child, prefix)
	}
}
*/

// Used as a workaround since we can't compare functions or their addresses
var fakeHandlerValue string

func fakeHandler(val string) Handler {
	return func(http.ResponseWriter, *http.Request, *Route, RouteParameters) {
		fakeHandlerValue = val
	}
}

type testRequests []struct {
	path       string
	nilHandler bool
	route      string
	ps         RouteParameters
}

func checkRequests(t *testing.T, tree *RouteNode, requests testRequests) {
	for _, request := range requests {
		route, ps, _ := tree.getValue(request.path)

		switch {
		case route == nil:
			if !request.nilHandler {
				t.Errorf("handle mismatch for route '%s': Expected non-nil handle", request.path)
			}
		case request.nilHandler:
			t.Errorf("handle mismatch for route '%s': Expected nil handle", request.path)
		default:
			route.Handler(nil, nil, nil, nil)
			if fakeHandlerValue != request.route {
				t.Errorf("handle mismatch for route '%s': Wrong handle (%s != %s)", request.path, fakeHandlerValue, request.route)
			}
		}

		if !reflect.DeepEqual(ps, request.ps) {
			t.Errorf("Params mismatch for route '%s' \n %#v vs. %#v\n", request.path, ps, request.ps)
		}
	}
}

func checkPriorities(t *testing.T, n *RouteNode) uint32 {
	var prio uint32
	for i := range n.Children {
		prio += checkPriorities(t, n.Children[i])
	}

	if n.Route != nil {
		prio++
	}

	if n.Priority != prio {
		t.Errorf(
			"priority mismatch for node '%s': is %d, should be %d",
			n.Path, n.Priority, prio,
		)
	}

	return prio
}

func checkMaxParams(t *testing.T, n *RouteNode) uint8 {
	var maxParams uint8
	for i := range n.Children {
		params := checkMaxParams(t, n.Children[i])
		if params > maxParams {
			maxParams = params
		}
	}
	if n.RouteNodeType > RouteNodeTypeRoot && !n.IsWildcard {
		maxParams++
	}

	if n.MaxParams != maxParams {
		t.Errorf(
			"maxParams mismatch for node '%s': is %d, should be %d",
			n.Path, n.MaxParams, maxParams,
		)
	}

	return maxParams
}

func TestCountParams(t *testing.T) {
	if countParams("/path/:param1/static/*catch-all") != 2 {
		t.Fail()
	}
	if countParams(strings.Repeat("/:param", 256)) != 255 {
		t.Fail()
	}
}

func TestTreeAddAndGet(t *testing.T) {
	tree := &RouteNode{}

	routes := [...]string{
		"/hi",
		"/contact",
		"/co",
		"/c",
		"/a",
		"/ab",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/α",
		"/β",
	}
	for _, route := range routes {
		tree.AddRoute("GET", route, fakeHandler(route))
	}

	//printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/a", false, "/a", nil},
		{"/", true, "", nil},
		{"/hi", false, "/hi", nil},
		{"/contact", false, "/contact", nil},
		{"/co", false, "/co", nil},
		{"/con", true, "", nil},  // key mismatch
		{"/cona", true, "", nil}, // key mismatch
		{"/no", true, "", nil},   // no matching child
		{"/ab", false, "/ab", nil},
		{"/α", false, "/α", nil},
		{"/β", false, "/β", nil},
	})

	checkPriorities(t, tree)
	checkMaxParams(t, tree)
}

func TestTreeWildcard(t *testing.T) {
	tree := &RouteNode{}

	routes := [...]string{
		"/",
		"/cmd/:tool/:sub",
		"/cmd/:tool/",
		"/src/*filepath",
		"/search/",
		"/search/:query",
		"/user_:name",
		"/user_:name/about",
		"/files/:dir/*filepath",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/info/:user/public",
		"/info/:user/project/:project",
	}
	for _, route := range routes {
		tree.AddRoute("GET", route, fakeHandler(route))
	}

	//printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/cmd/test/", false, "/cmd/:tool/", RouteParameters{"tool": "test"}},
		{"/cmd/test", true, "", RouteParameters{"tool": "test"}},
		{"/cmd/test/3", false, "/cmd/:tool/:sub", RouteParameters{"tool": "test", "sub": "3"}},
		{"/src/", false, "/src/*filepath", RouteParameters{"filepath": "/"}},
		{"/src/some/file.png", false, "/src/*filepath", RouteParameters{"filepath": "/some/file.png"}},
		{"/search/", false, "/search/", nil},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", RouteParameters{"query": "someth!ng+in+ünìcodé"}},
		{"/search/someth!ng+in+ünìcodé/", true, "", RouteParameters{"query": "someth!ng+in+ünìcodé"}},
		{"/user_gopher", false, "/user_:name", RouteParameters{"name": "gopher"}},
		{"/user_gopher/about", false, "/user_:name/about", RouteParameters{"name": "gopher"}},
		{"/files/js/inc/framework.js", false, "/files/:dir/*filepath", RouteParameters{"dir": "js", "filepath": "/inc/framework.js"}},
		{"/info/gordon/public", false, "/info/:user/public", RouteParameters{"user": "gordon"}},
		{"/info/gordon/project/go", false, "/info/:user/project/:project", RouteParameters{"user": "gordon", "project": "go"}},
	})

	checkPriorities(t, tree)
	checkMaxParams(t, tree)
}

func catchPanic(testFunc func()) (recv interface{}) {
	defer func() {
		recv = recover()
	}()

	testFunc()
	return
}

type testRoute struct {
	path     string
	conflict bool
}

func testRoutes(t *testing.T, routes []testRoute) {
	tree := &RouteNode{}

	for _, route := range routes {
		recv := catchPanic(func() {
			tree.AddRoute("GET", route.path, nil)
		})

		if route.conflict {
			if recv == nil {
				t.Errorf("no panic for conflicting route '%s'", route.path)
			}
		} else if recv != nil {
			t.Errorf("unexpected panic for route '%s': %v", route.path, recv)
		}
	}

	//printChildren(tree, "")
}

func TestTreeWildcardConflict(t *testing.T) {
	routes := []testRoute{
		{"/cmd/:tool/:sub", false},
		{"/cmd/vet", true},
		{"/src/*filepath", false},
		{"/src/*filepathx", true},
		{"/src/", true},
		{"/src1/", false},
		{"/src1/*filepath", true},
		{"/src2*filepath", true},
		{"/search/:query", false},
		{"/search/invalid", true},
		{"/user_:name", false},
		{"/user_x", true},
		{"/user_:name", true},
		{"/id:id", false},
		{"/id/:id", true},
	}
	testRoutes(t, routes)
}

func TestTreeChildConflict(t *testing.T) {
	routes := []testRoute{
		{"/cmd/vet", false},
		{"/cmd/:tool/:sub", true},
		{"/src/AUTHORS", false},
		{"/src/*filepath", true},
		{"/user_x", false},
		{"/user_:name", true},
		{"/id/:id", false},
		{"/id:id", true},
		{"/:id", true},
		{"/*filepath", true},
	}
	testRoutes(t, routes)
}

func TestTreeDupliatePath(t *testing.T) {
	tree := &RouteNode{}

	routes := [...]string{
		"/",
		"/doc/",
		"/src/*filepath",
		"/search/:query",
		"/user_:name",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.AddRoute("GET", route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}

		// Add again
		recv = catchPanic(func() {
			tree.AddRoute("GET", route, nil)
		})
		if recv == nil {
			t.Fatalf("no panic while inserting duplicate route '%s", route)
		}
	}

	//printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/doc/", false, "/doc/", nil},
		{"/src/some/file.png", false, "/src/*filepath", RouteParameters{"filepath": "/some/file.png"}},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", RouteParameters{"query": "someth!ng+in+ünìcodé"}},
		{"/user_gopher", false, "/user_:name", RouteParameters{"name": "gopher"}},
	})
}

func TestEmptyWildcardName(t *testing.T) {
	tree := &RouteNode{}

	routes := [...]string{
		"/user:",
		"/user:/",
		"/cmd/:/",
		"/src/*",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.AddRoute("GET", route, nil)
		})
		if recv == nil {
			t.Fatalf("no panic while inserting route with empty wildcard name '%s", route)
		}
	}
}

func TestTreeCatchAllConflict(t *testing.T) {
	routes := []testRoute{
		{"/src/*filepath/x", true},
		{"/src2/", false},
		{"/src2/*filepath/x", true},
	}
	testRoutes(t, routes)
}

func TestTreeCatchAllConflictRoot(t *testing.T) {
	routes := []testRoute{
		{"/", false},
		{"/*filepath", true},
	}
	testRoutes(t, routes)
}

func TestTreeDoubleWildcard(t *testing.T) {
	const panicMsg = "only one wildcard per path segment is allowed"

	routes := [...]string{
		"/:foo:bar",
		"/:foo:bar/",
		"/:foo*bar",
	}

	for _, route := range routes {
		tree := &RouteNode{}
		recv := catchPanic(func() {
			tree.AddRoute("GET", route, nil)
		})

		if rs, ok := recv.(string); !ok || !strings.HasPrefix(rs, panicMsg) {
			t.Fatalf(`"Expected panic "%s" for route '%s', got "%v"`, panicMsg, route, recv)
		}
	}
}

/*func TestTreeDuplicateWildcard(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/:id/:name/:id",
	}
	for _, route := range routes {
		...
	}
}*/

func TestTreeTrailingSlashRedirect(t *testing.T) {
	tree := &RouteNode{}

	routes := [...]string{
		"/hi",
		"/b/",
		"/search/:query",
		"/cmd/:tool/",
		"/src/*filepath",
		"/x",
		"/x/y",
		"/y/",
		"/y/z",
		"/0/:id",
		"/0/:id/1",
		"/1/:id/",
		"/1/:id/2",
		"/aa",
		"/a/",
		"/admin",
		"/admin/:category",
		"/admin/:category/:page",
		"/doc",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/no/a",
		"/no/b",
		"/api/hello/:name",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.AddRoute("GET", route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}

	//printChildren(tree, "")

	tsrRoutes := [...]string{
		"/hi/",
		"/b",
		"/search/gopher/",
		"/cmd/vet",
		"/src",
		"/x/",
		"/y",
		"/0/go/",
		"/1/go",
		"/a",
		"/admin/",
		"/admin/config/",
		"/admin/config/permissions/",
		"/doc/",
	}
	for _, route := range tsrRoutes {
		handler, _, tsr := tree.getValue(route)
		if handler != nil {
			t.Fatalf("non-nil handler for TSR route '%s", route)
		} else if !tsr {
			t.Errorf("expected TSR recommendation for route '%s'", route)
		}
	}

	noTsrRoutes := [...]string{
		"/",
		"/no",
		"/no/",
		"/_",
		"/_/",
		"/api/world/abc",
	}
	for _, route := range noTsrRoutes {
		handler, _, tsr := tree.getValue(route)
		if handler != nil {
			t.Fatalf("non-nil handler for No-TSR route '%s", route)
		} else if tsr {
			t.Errorf("expected no TSR recommendation for route '%s'", route)
		}
	}
}

func TestTreeRootTrailingSlashRedirect(t *testing.T) {
	tree := &RouteNode{}

	recv := catchPanic(func() {
		tree.AddRoute("GET", "/:test", fakeHandler("/:test"))
	})
	if recv != nil {
		t.Fatalf("panic inserting test route: %v", recv)
	}

	handler, _, tsr := tree.getValue("/")
	if handler != nil {
		t.Fatalf("non-nil handler")
	} else if tsr {
		t.Errorf("expected no TSR recommendation")
	}
}

func TestTreeFindCaseInsensitivePath(t *testing.T) {
	tree := &RouteNode{}

	routes := [...]string{
		"/hi",
		"/b/",
		"/ABC/",
		"/search/:query",
		"/cmd/:tool/",
		"/src/*filepath",
		"/x",
		"/x/y",
		"/y/",
		"/y/z",
		"/0/:id",
		"/0/:id/1",
		"/1/:id/",
		"/1/:id/2",
		"/aa",
		"/a/",
		"/doc",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/doc/go/away",
		"/no/a",
		"/no/b",
	}

	for _, route := range routes {
		recv := catchPanic(func() {
			tree.AddRoute("GET", route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}

	// Check out == in for all registered routes
	// With fixTrailingSlash = true
	for _, route := range routes {
		out, found := tree.findCaseInsensitivePath(route, true)
		if !found {
			t.Errorf("Route '%s' not found!", route)
		} else if string(out) != route {
			t.Errorf("Wrong result for route '%s': %s", route, string(out))
		}
	}
	// With fixTrailingSlash = false
	for _, route := range routes {
		out, found := tree.findCaseInsensitivePath(route, false)
		if !found {
			t.Errorf("Route '%s' not found!", route)
		} else if string(out) != route {
			t.Errorf("Wrong result for route '%s': %s", route, string(out))
		}
	}

	tests := []struct {
		in    string
		out   string
		found bool
		slash bool
	}{
		{"/HI", "/hi", true, false},
		{"/HI/", "/hi", true, true},
		{"/B", "/b/", true, true},
		{"/B/", "/b/", true, false},
		{"/abc", "/ABC/", true, true},
		{"/abc/", "/ABC/", true, false},
		{"/aBc", "/ABC/", true, true},
		{"/aBc/", "/ABC/", true, false},
		{"/abC", "/ABC/", true, true},
		{"/abC/", "/ABC/", true, false},
		{"/SEARCH/QUERY", "/search/QUERY", true, false},
		{"/SEARCH/QUERY/", "/search/QUERY", true, true},
		{"/CMD/TOOL/", "/cmd/TOOL/", true, false},
		{"/CMD/TOOL", "/cmd/TOOL/", true, true},
		{"/SRC/FILE/PATH", "/src/FILE/PATH", true, false},
		{"/x/Y", "/x/y", true, false},
		{"/x/Y/", "/x/y", true, true},
		{"/X/y", "/x/y", true, false},
		{"/X/y/", "/x/y", true, true},
		{"/X/Y", "/x/y", true, false},
		{"/X/Y/", "/x/y", true, true},
		{"/Y/", "/y/", true, false},
		{"/Y", "/y/", true, true},
		{"/Y/z", "/y/z", true, false},
		{"/Y/z/", "/y/z", true, true},
		{"/Y/Z", "/y/z", true, false},
		{"/Y/Z/", "/y/z", true, true},
		{"/y/Z", "/y/z", true, false},
		{"/y/Z/", "/y/z", true, true},
		{"/Aa", "/aa", true, false},
		{"/Aa/", "/aa", true, true},
		{"/AA", "/aa", true, false},
		{"/AA/", "/aa", true, true},
		{"/aA", "/aa", true, false},
		{"/aA/", "/aa", true, true},
		{"/A/", "/a/", true, false},
		{"/A", "/a/", true, true},
		{"/DOC", "/doc", true, false},
		{"/DOC/", "/doc", true, true},
		{"/NO", "", false, true},
		{"/DOC/GO", "", false, true},
	}
	// With fixTrailingSlash = true
	for _, test := range tests {
		out, found := tree.findCaseInsensitivePath(test.in, true)
		if found != test.found || (found && (string(out) != test.out)) {
			t.Errorf("Wrong result for '%s': got %s, %t; want %s, %t",
				test.in, string(out), found, test.out, test.found)
			return
		}
	}
	// With fixTrailingSlash = false
	for _, test := range tests {
		out, found := tree.findCaseInsensitivePath(test.in, false)
		if test.slash {
			if found { // test needs a trailingSlash fix. It must not be found!
				t.Errorf("Found without fixTrailingSlash: %s; got %s", test.in, string(out))
			}
		} else {
			if found != test.found || (found && (string(out) != test.out)) {
				t.Errorf("Wrong result for '%s': got %s, %t; want %s, %t",
					test.in, string(out), found, test.out, test.found)
				return
			}
		}
	}
}

func TestTreeInvalidNodeType(t *testing.T) {
	const panicMsg = "invalid node type"

	tree := &RouteNode{}
	tree.AddRoute("GET", "/", fakeHandler("/"))
	tree.AddRoute("GET", "/:page", fakeHandler("/:page"))

	// set invalid node type
	tree.Children[0].RouteNodeType = 42

	// normal lookup
	recv := catchPanic(func() {
		tree.getValue("/test")
	})
	if rs, ok := recv.(string); !ok || rs != panicMsg {
		t.Fatalf("Expected panic '"+panicMsg+"', got '%v'", recv)
	}

	// case-insensitive lookup
	recv = catchPanic(func() {
		tree.findCaseInsensitivePath("/test", true)
	})
	if rs, ok := recv.(string); !ok || rs != panicMsg {
		t.Fatalf("Expected panic '"+panicMsg+"', got '%v'", recv)
	}
}
