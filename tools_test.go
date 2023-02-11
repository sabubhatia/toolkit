package toolkit

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestToolsRandomString(t *testing.T) {
	var testTools Tools

	n := 10
	s := testTools.RandomString(n)
	if len(s) != n {
		t.Errorf("Unexpected length. Expected: %d, Got: %d", n, len(s))
	}
}

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{name: "allowed no rename", allowedTypes: []string{"image/png", "image/jpeg"}, renameFile: false, errorExpected: false},
	{name: "allowed rename", allowedTypes: []string{"image/png", "image/jpeg"}, renameFile: true, errorExpected: false},
	{name: "File type not allowed", allowedTypes: []string{"image/png"}, renameFile: true, errorExpected: true},
}

func TestToolsUploadFile(t *testing.T) {
	for _, e := range uploadTests {
		// set up a pipe to avoid buffering
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer func() {
				writer.Close()
			}()
			defer wg.Done()
			part, err := writer.CreateFormFile("file", "./testdata/toby.jpg")
			if err != nil {
				t.Error(err.Error())
				return
			}

			f, err := os.Open("./testdata/toby.jpg")
			if err != nil {
				t.Error(err.Error())
				return
			}
			defer f.Close()
			img, _, err := image.Decode(f)
			if err != nil {
				t.Error("Unable to decode image ", err.Error())
			}
			if err := jpeg.Encode(part, img, nil); err != nil {
				t.Error("Unable to encode image ", err.Error())
			}
		}()
		request := httptest.NewRequest("POST", "/", pr)
		request.Header.Set("content-type", writer.FormDataContentType())
		testTool := Tools{AllowedTypes: e.allowedTypes}
		uploadedFiles, err := testTool.UploadFile(request, "./testdata/upload", e.renameFile)
		if err != nil && !e.errorExpected {
			t.Error(err)
		}

		if !e.errorExpected {
			nf := fmt.Sprintf("./testdata/upload/%s", uploadedFiles[0].NewFileName)
			if _, err := os.Stat(nf); err != nil && err == fs.ErrNotExist {
				t.Errorf("%s, expected the file to exist", e.name)
			}

			// clean up
			os.Remove(nf)
		}

		if e.errorExpected && err == nil {
			t.Errorf("%s expected an error but none exists", e.name)
		}
		wg.Wait()
	}
}

func TestToolsUploadOneFile(t *testing.T) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		defer writer.Close()
		part, err := writer.CreateFormFile("file", "./testdata/toby.jpg")
		if err != nil {
			t.Error(err)
			return
		}
		f, err := os.Open("./testdata/toby.jpg")
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			t.Error(err)
			return
		}

		err = jpeg.Encode(part, img, nil)
		if err != nil {
			t.Error(err)
			return
		}
	}()
	r := httptest.NewRequest("POST", "/", pr)
	r.Header.Add("content-type", writer.FormDataContentType())
	testTool := Tools{AllowedTypes: []string{"image/jpeg"}}
	uploadedFile, err := testTool.UploadOneFile(r, "./testdata/upload", true)
	if err != nil {
		t.Error(err)
		return
	}

	nf := fmt.Sprintf("./testdata/upload/%s", uploadedFile.NewFileName)
	_, err = os.Stat(nf)
	if err != nil && err == fs.ErrNotExist {
		t.Error("File Expected to exist")
		return
	}
	os.Remove(nf)
}

func TestToolsCreateDirIfNotExists(t *testing.T) {
	var testTool Tools
	d := "./testdata/mydir"
	os.RemoveAll(d)
	err := testTool.CreateDirIfNotExists(d)
	if err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(d); errors.Is(err, fs.ErrNotExist) {
		t.Error("expected the directory to exist ", err)
	}
	os.RemoveAll(d)
}

var slugTests = []struct {
	name        string
	s           string
	expected    string
	errExpected bool
}{
	{name: "valid string", s: "Now is the time for black", expected: "now-is-the-time-for-black", errExpected: false},
	{name: "empty string", s: "", expected: "", errExpected: true},
	{name: "complex string", s: "NOW IS THE time&$___&$& for><><>&!!black%^%&%*&)123", expected: "now-is-the-time-for-black", errExpected: false},
	{name: "thai string", s: "สวัสดีชาวโลก", expected: "", errExpected: true,},
	{name: "thai string and roman characters", s: "hello blackสวัสดีชาวโลก", expected: "hello-black", errExpected: false},
}

func TestToolsSlugify(t *testing.T) {
	var tools Tools
	for _, e := range slugTests {
		slug, err := tools.Slugify(e.s)
		if err != nil {
			if e.errExpected {
				continue
			}
			if !e.errExpected {
				t.Errorf("%s: error received when none expected %s", e.name, err)
				continue
			}
		}
		if !e.errExpected && !strings.EqualFold(slug, e.expected) {
			t.Errorf("%s after slugify expected %s got %s", e.name, e.expected, slug)
			continue
		}

		if e.errExpected {
			t.Errorf("%s error expected none received", e.name)
			continue
		}
	}
}
