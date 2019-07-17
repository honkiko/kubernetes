/*
Copyright 2014 The Kubernetes Authors.

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

package cmd

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

type FileType int

const (
	RegularFile FileType = 0
	SymLink     FileType = 1
	RegexFile   FileType = 2
)

func TestExtractFileSpec(t *testing.T) {
	tests := []struct {
		spec              string
		expectedPod       string
		expectedNamespace string
		expectedFile      string
		expectErr         bool
	}{
		{
			spec:              "namespace/pod:/some/file",
			expectedPod:       "pod",
			expectedNamespace: "namespace",
			expectedFile:      "/some/file",
		},
		{
			spec:         "pod:/some/file",
			expectedPod:  "pod",
			expectedFile: "/some/file",
		},
		{
			spec:         "/some/file",
			expectedFile: "/some/file",
		},
		{
			spec:      "some:bad:spec",
			expectErr: true,
		},
	}
	for _, test := range tests {
		spec, err := extractFileSpec(test.spec)
		if test.expectErr && err == nil {
			t.Errorf("unexpected non-error")
			continue
		}
		if err != nil && !test.expectErr {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if spec.PodName != test.expectedPod {
			t.Errorf("expected: %s, saw: %s", test.expectedPod, spec.PodName)
		}
		if spec.PodNamespace != test.expectedNamespace {
			t.Errorf("expected: %s, saw: %s", test.expectedNamespace, spec.PodNamespace)
		}
		if spec.File != test.expectedFile {
			t.Errorf("expected: %s, saw: %s", test.expectedFile, spec.File)
		}
	}
}

func TestGetPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "/foo/bar",
			expected: "foo/bar",
		},
		{
			input:    "foo/bar",
			expected: "foo/bar",
		},
	}
	for _, test := range tests {
		out := getPrefix(test.input)
		if out != test.expected {
			t.Errorf("expected: %s, saw: %s", test.expected, out)
		}
	}
}

func TestTarUntar(t *testing.T) {
	dir, err := ioutil.TempDir("", "input")
	checkErr(t, err)
	dir2, err := ioutil.TempDir("", "output")
	checkErr(t, err)
	dir3, err := ioutil.TempDir("", "dir")
	checkErr(t, err)

	dir = dir + "/"
	defer func() {
		os.RemoveAll(dir)
		os.RemoveAll(dir2)
		os.RemoveAll(dir3)
	}()

	files := []struct {
		name     string
		nameList []string
		data     string
		omitted  bool
		fileType FileType
	}{
		{
			name:     "foo",
			data:     "foobarbaz",
			fileType: RegularFile,
		},
		{
			name:     "dir/blah",
			data:     "bazblahfoo",
			fileType: RegularFile,
		},
		{
			name:     "some/other/directory/",
			data:     "with more data here",
			fileType: RegularFile,
		},
		{
			name:     "blah",
			data:     "same file name different data",
			fileType: RegularFile,
		},
		{
			name:     "gakki",
			data:     "tmp/gakki",
			fileType: SymLink,
		},
		{
			name:     "relative_to_dest",
			data:     path.Join(dir2, "foo"),
			fileType: SymLink,
		},
		{
			name:     "tricky_relative",
			data:     path.Join(dir3, "xyz"),
			omitted:  true,
			fileType: SymLink,
		},
		{
			name:     "absolute_path",
			data:     "/tmp/gakki",
			omitted:  true,
			fileType: SymLink,
		},
		{
			name:     "blah*",
			nameList: []string{"blah1", "blah2"},
			data:     "regexp file name",
			fileType: RegexFile,
		},
	}

	for _, file := range files {
		filepath := path.Join(dir, file.name)
		if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if file.fileType == RegularFile {
			createTmpFile(t, filepath, file.data)
		} else if file.fileType == SymLink {
			err := os.Symlink(file.data, filepath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		} else if file.fileType == RegexFile {
			for _, fileName := range file.nameList {
				createTmpFile(t, path.Join(dir, fileName), file.data)
			}
		} else {
			t.Fatalf("unexpected file type: %v", file)
		}
	}

	writer := &bytes.Buffer{}
	if err := makeTar(dir, dir, writer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reader := bytes.NewBuffer(writer.Bytes())
	if err := untarAll(reader, dir2, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, file := range files {
		absPath := filepath.Join(dir2, strings.TrimPrefix(dir, os.TempDir()))
		filePath := filepath.Join(absPath, file.name)

		if file.fileType == RegularFile {
			cmpFileData(t, filePath, file.data)
		} else if file.fileType == SymLink {
			dest, err := os.Readlink(filePath)
			if file.omitted {
				if err != nil && strings.Contains(err.Error(), "no such file or directory") {
					continue
				}
				t.Fatalf("expected to omit symlink for %s", filePath)
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if file.data != dest {
				t.Fatalf("expected: %s, saw: %s", file.data, dest)
			}
		} else if file.fileType == RegexFile {
			for _, fileName := range file.nameList {
				cmpFileData(t, path.Join(dir, fileName), file.data)
			}
		} else {
			t.Fatalf("unexpected file type: %v", file)
		}
	}
}

func TestTarDestinationName(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "input")
	dir2, err2 := ioutil.TempDir(os.TempDir(), "output")
	if err != nil || err2 != nil {
		t.Errorf("unexpected error: %v | %v", err, err2)
		t.FailNow()
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("Unexpected error cleaning up: %v", err)
		}
		if err := os.RemoveAll(dir2); err != nil {
			t.Errorf("Unexpected error cleaning up: %v", err)
		}
	}()

	files := []struct {
		name string
		data string
	}{
		{
			name: "foo",
			data: "foobarbaz",
		},
		{
			name: "dir/blah",
			data: "bazblahfoo",
		},
		{
			name: "some/other/directory",
			data: "with more data here",
		},
		{
			name: "blah",
			data: "same file name different data",
		},
	}

	// ensure files exist on disk
	for _, file := range files {
		filepath := path.Join(dir, file.name)
		if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
			t.Errorf("unexpected error: %v", err)
			t.FailNow()
		}
		f, err := os.Create(filepath)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			t.FailNow()
		}
		defer f.Close()
		if _, err := io.Copy(f, bytes.NewBuffer([]byte(file.data))); err != nil {
			t.Errorf("unexpected error: %v", err)
			t.FailNow()
		}
	}

	reader, writer := io.Pipe()
	go func() {
		if err := makeTar(dir, dir2, writer); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}()

	tarReader := tar.NewReader(reader)
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Errorf("unexpected error: %v", err)
			t.FailNow()
		}

		if !strings.HasPrefix(hdr.Name, path.Base(dir2)) {
			t.Errorf("expected %q as destination filename prefix, saw: %q", path.Base(dir2), hdr.Name)
		}
	}
}

func TestBadTar(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "dest")
	if err != nil {
		t.Errorf("unexpected error: %v ", err)
		t.FailNow()
	}
	defer os.RemoveAll(dir)

	// More or less cribbed from https://golang.org/pkg/archive/tar/#example__minimal
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var files = []struct {
		name string
		body string
	}{
		{"/prefix/foo/bar/../../home/bburns/names.txt", "Down and back"},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.name,
			Mode: 0600,
			Size: int64(len(file.body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Errorf("unexpected error: %v ", err)
			t.FailNow()
		}
		if _, err := tw.Write([]byte(file.body)); err != nil {
			t.Errorf("unexpected error: %v ", err)
			t.FailNow()
		}
	}
	if err := tw.Close(); err != nil {
		t.Errorf("unexpected error: %v ", err)
		t.FailNow()
	}

	if err := untarAll(&buf, dir, "/prefix"); err != nil {
		t.Errorf("unexpected error: %v ", err)
		t.FailNow()
	}

	for _, file := range files {
		_, err := os.Stat(path.Join(dir, path.Clean(file.name[len("/prefix"):])))
		if err != nil {
			t.Errorf("Error finding file: %v", err)
		}
	}

}

func TestIsDestRelative(t *testing.T) {
	tests := []struct {
		base     string
		dest     string
		relative bool
	}{
		{
			base:     "/dir",
			dest:     "../link",
			relative: false,
		},
		{
			base:     "/dir",
			dest:     "../../link",
			relative: false,
		},
		{
			base:     "/dir",
			dest:     "/link",
			relative: false,
		},
		{
			base:     "/dir",
			dest:     "link",
			relative: true,
		},
		{
			base:     "/dir",
			dest:     "int/file/link",
			relative: true,
		},
		{
			base:     "/dir",
			dest:     "int/../link",
			relative: true,
		},
		{
			base:     "/dir",
			dest:     "/dir/link",
			relative: true,
		},
		{
			base:     "/dir",
			dest:     "/dir/int/../link",
			relative: true,
		},
		{
			base:     "/dir",
			dest:     "/dir/../../link",
			relative: false,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			if test.relative != isDestRelative(test.base, test.dest) {
				t.Errorf("unexpected result for: base %q, dest %q", test.base, test.dest)
			}
		})
	}
}

func TestUntar(t *testing.T) {
	testdir, err := ioutil.TempDir("", "test-untar")
	require.NoError(t, err)
	defer os.RemoveAll(testdir)
	t.Logf("Test base: %s", testdir)

	const (
		dest = "base"
	)

	type file struct {
		path       string
		linkTarget string // For link types
		expected   string // Expect to find the file here (or not, if empty)
	}
	files := []file{{
		// Absolute file within dest
		path:     filepath.Join(testdir, dest, "abs"),
		expected: filepath.Join(testdir, dest, testdir, dest, "abs"),
	}, { // Absolute file outside dest
		path:     filepath.Join(testdir, "abs-out"),
		expected: filepath.Join(testdir, dest, testdir, "abs-out"),
	}, { // Absolute nested file within dest
		path:     filepath.Join(testdir, dest, "nested/nest-abs"),
		expected: filepath.Join(testdir, dest, testdir, dest, "nested/nest-abs"),
	}, { // Absolute nested file outside dest
		path:     filepath.Join(testdir, dest, "nested/../../nest-abs-out"),
		expected: filepath.Join(testdir, dest, testdir, "nest-abs-out"),
	}, { // Relative file inside dest
		path:     "relative",
		expected: filepath.Join(testdir, dest, "relative"),
	}, { // Relative file outside dest
		path:     "../unrelative",
		expected: "",
	}, { // Nested relative file inside dest
		path:     "nested/nest-rel",
		expected: filepath.Join(testdir, dest, "nested/nest-rel"),
	}, { // Nested relative file outside dest
		path:     "nested/../../nest-unrelative",
		expected: "",
	}}

	mkExpectation := func(expected, suffix string) string {
		if expected == "" {
			return ""
		}
		return expected + suffix
	}
	mkBacktickExpectation := func(expected, suffix string) string {
		dir, _ := filepath.Split(filepath.Clean(expected))
		if len(strings.Split(dir, string(os.PathSeparator))) <= 1 {
			return ""
		}
		return expected + suffix
	}
	links := []file{}
	for _, f := range files {
		links = append(links, file{
			path:       f.path + "-innerlink",
			linkTarget: "link-target",
			expected:   mkExpectation(f.expected, "-innerlink"),
		}, file{
			path:       f.path + "-innerlink-abs",
			linkTarget: filepath.Join(testdir, dest, "link-target"),
			expected:   mkExpectation(f.expected, "-innerlink-abs"),
		}, file{
			path:       f.path + "-outerlink",
			linkTarget: filepath.Join(backtick(f.path), "link-target"),
			expected:   mkBacktickExpectation(f.expected, "-outerlink"),
		}, file{
			path:       f.path + "-outerlink-abs",
			linkTarget: filepath.Join(testdir, "link-target"),
			expected:   "",
		})
	}
	files = append(files, links...)

	// Test back-tick escaping through a symlink.
	files = append(files,
		file{
			path:       "nested/again/back-link",
			linkTarget: "../../nested",
			expected:   filepath.Join(testdir, dest, "nested/again/back-link"),
		},
		file{
			path:     "nested/again/back-link/../../../back-link-file",
			expected: filepath.Join(testdir, dest, "back-link-file"),
		})

	// Test chaining back-tick symlinks.
	files = append(files,
		file{
			path:       "nested/back-link-first",
			linkTarget: "../",
			expected:   filepath.Join(testdir, dest, "nested/back-link-first"),
		},
		file{
			path:       "nested/back-link-first/back-link-second",
			linkTarget: "../",
			expected:   filepath.Join(testdir, dest, "back-link-second"),
		},
		file{
			// This case is chaining together symlinks that step back, so that
			// if you just look at the target relative to the path it appears
			// inside the destination directory, but if you actually follow each
			// step of the path you end up outside the destination directory.
			path:       "nested/back-link-first/back-link-second/back-link-term",
			linkTarget: "",
			expected:   "",
		})

	files = append(files,
		file{ // Relative directory path with terminating /
			path:     "direct/dir/",
			expected: "",
		})

	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	expectations := map[string]bool{}
	for _, f := range files {
		if f.expected != "" {
			expectations[f.expected] = false
		}
		if f.linkTarget == "" {
			hdr := &tar.Header{
				Name: f.path,
				Mode: 0666,
				Size: int64(len(f.path)),
			}
			require.NoError(t, tw.WriteHeader(hdr), f.path)
			if !strings.HasSuffix(f.path, "/") {
				_, err := tw.Write([]byte(f.path))
				require.NoError(t, err, f.path)
			}
		} else {
			hdr := &tar.Header{
				Name:     f.path,
				Mode:     int64(0777 | os.ModeSymlink),
				Typeflag: tar.TypeSymlink,
				Linkname: f.linkTarget,
			}
			require.NoError(t, tw.WriteHeader(hdr), f.path)
		}
	}
	tw.Close()

	require.NoError(t, untarAll(buf, filepath.Join(testdir, dest), ""))

	filepath.Walk(testdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil // Ignore directories.
		}
		if _, ok := expectations[path]; !ok {
			t.Errorf("Unexpected file at %s", path)
		} else {
			expectations[path] = true
		}
		return nil
	})
	for path, found := range expectations {
		if !found {
			t.Errorf("Missing expected file %s", path)
		}
	}
}

// backtick returns a path to one directory up from the target
func backtick(target string) string {
	rel, _ := filepath.Rel(filepath.Dir(target), "../")
	return rel
}


func TestClean(t *testing.T) {
	tests := []struct {
		input   string
		cleaned string
	}{
		{
			"../../../tmp/foo",
			"/tmp/foo",
		},
		{
			"/../../../tmp/foo",
			"/tmp/foo",
		},
	}

	for _, test := range tests {
		out := clean(test.input)
		if out != test.cleaned {
			t.Errorf("Expected: %s, saw %s", test.cleaned, out)
		}
	}
}

func checkErr(t *testing.T, err error) {
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		t.FailNow()
	}
}

func createTmpFile(t *testing.T, filepath, data string) {
	f, err := os.Create(filepath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, bytes.NewBuffer([]byte(data))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func cmpFileData(t *testing.T, filePath, data string) {
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer f.Close()
	buff := &bytes.Buffer{}
	if _, err := io.Copy(buff, f); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if data != string(buff.Bytes()) {
		t.Fatalf("expected: %s, saw: %s", data, string(buff.Bytes()))
	}
}