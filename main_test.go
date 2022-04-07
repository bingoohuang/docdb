package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getPath(t *testing.T) {
	tests := []struct {
		object        H
		path          []string
		expectedValue any
		expectedOk    bool
	}{
		{object: H{"a": H{"b": 1}}, path: []string{"a", "b"}, expectedValue: 1, expectedOk: true},
		{object: H{"a": H{"b": 1}}, path: []string{"a", "c"}},
	}

	for _, test := range tests {
		v, ok := getPath(test.object, test.path)
		assert.Equal(t, test.expectedValue, v)
		assert.Equal(t, test.expectedOk, ok)
	}
}

func Test_lexString(t *testing.T) {
	tests := []struct {
		input          string
		index          int
		expectedString string
		expectedIndex  int
		expectedErr    error
	}{
		{input: "a.b:c", expectedString: "a.b", expectedIndex: 3},
		{input: `"a b : . 2":12`, expectedString: "a b : . 2", expectedIndex: 11},
		{input: ` a:2`, expectedErr: fmt.Errorf("no string found")},
		{input: ` a:2`, index: 1, expectedString: "a", expectedIndex: 2},
	}

	for _, test := range tests {
		s, outIndex, err := lexString([]rune(test.input), test.index)
		assert.Equal(t, test.expectedString, s)
		assert.Equal(t, test.expectedIndex, outIndex)
		assert.Equal(t, test.expectedErr, err)
	}
}

func Test_parseQuery(t *testing.T) {
	tests := []struct {
		q             string
		expectedQuery query
		expectedErr   error
	}{
		{q: "a.b:1 c:2", expectedQuery: query{ands: []queryComparison{{key: []string{"a", "b"}, value: "1", op: "="}, {key: []string{"c"}, value: "2", op: "="}}}},
		{q: "a:1", expectedQuery: query{ands: []queryComparison{{key: []string{"a"}, value: "1", op: "="}}}},
		{q: `" a ":" n "`, expectedQuery: query{ands: []queryComparison{{key: []string{" a "}, value: " n ", op: "="}}}},
		{q: "", expectedQuery: query{}},
	}

	for _, test := range tests {
		query, err := parseQuery(test.q)
		if query == nil {
			fmt.Println(test, err)
		}
		assert.Equal(t, test.expectedQuery, *query)
		assert.Equal(t, test.expectedErr, err)
	}
}

func Test_getPathValues(t *testing.T) {
	tests := []struct {
		obj         H
		prefix      string
		expectedPvs []string
	}{
		{obj: H{"a": 2, "b": 4, "c": "hey im here"}, expectedPvs: []string{"a=2", "b=4", "c=hey im here"}},
		{obj: H{"a": H{"12": "19"}}, expectedPvs: []string{"a.12=19"}},
	}

	for _, test := range tests {
		pvs := getPathValues(test.obj, test.prefix)
		assert.Equal(t, len(test.expectedPvs), len(pvs))
		assert.Equal(t, test.expectedPvs, pvs)
	}
}
