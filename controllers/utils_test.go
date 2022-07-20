/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"reflect"
	"testing"
)

func TestGeneratePodArgs(t *testing.T) {
	// TODO
}

func TestHasDifferentArguments(t *testing.T) {
	// TODO
}

func TestGenerateImage(t *testing.T) {
	rep := "repository/image"
	version := "version"
	expected := "repository/image:version"
	result := generateImage(rep, version)

	if expected != result {
		t.Errorf("generateImage(%v, %v) returned %v but expected %v", rep, version, result, expected)
	}
}

func TestMergeLabels(t *testing.T) {
	origin := map[string]string{
		"app": "ngnix",
	}
	toadd := map[string]string{
		"key": "value",
	}
	expected := map[string]string{
		"app": "ngnix",
		"key": "value",
	}

	result := mergeLabels(origin, toadd)

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("mergeLabels(%v, %v) returned %v but expected %v", origin, toadd, result, expected)
	}
}
