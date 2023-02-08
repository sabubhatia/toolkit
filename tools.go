package toolkit

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_+"

// Tools is the type used to instantiate this module. Any variable of this type will have access to all the methods 
// with the receiver *Tools
type Tools struct {
	MaxFileSize int
	AllowedTypes []string
}

// RandomString returns a string of randomn characters of length n, using randomStringSource 
// as the source for teh characters of the string
func (t * Tools) RandomString(n int) string {
	s, r := make([]rune, n), []rune(randomStringSource)

	for i := range s {
		p, err := rand.Prime(rand.Reader, len(r))
		if err != nil {
			log.Fatal("Unexpected error ", err)
		}		
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}

	return string(s)
}


// UploadedFile is a struct to store information about an file that has been uploaded.
type UploadedFile struct {
	NewFileName string
	OriginalFileName string
	FileSize int64

}

// UploadOneFile is a convenience function that is used to upload just one single file. This simply calls the more
// general UploadFile() function. 
func (t *Tools) UploadOneFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	uploadedFile, err := t.UploadFile(r, uploadDir, renameFile)
	if err != nil {
		return nil, err
	}

	return uploadedFile[0], nil
}

// UploadFile reads and loads files to a specified directory. If rename is true it uses the RandomString()
// function to generate a new file name. The extension of the file is always the same as that of the original file name.
func (t *Tools) UploadFile(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	var upLoadedFiles []*UploadedFile

	if t.MaxFileSize == 0 {
		t.MaxFileSize = 1024 * 1024 * 1024  // 1 gb approximately
	}
	err := r.ParseMultipartForm(int64(t.MaxFileSize))
	if err != nil {
		log.Println(r)
		log.Fatal("Fatal:", err)
		return nil, errors.New(err.Error())
	}
	
	for _, fHeaders := range r.MultipartForm.File {
		for _, hdrs := range fHeaders {
			upLoadedFiles, err = func(uploadedFiles []*UploadedFile) ([]*UploadedFile, error) {
				inFile, err := hdrs.Open()
				if err != nil {
					return nil, err
				}
				defer inFile.Close()

				buf := make([]byte, 512)
				_, err = inFile.Read(buf)
				if err != nil {
					return nil, err
				}

				// Check to see of the file type is permitted.
				allowed := false
				fileType := http.DetectContentType(buf)

				if len(t.AllowedTypes) > 0 {
					for _, x := range t.AllowedTypes {
						if strings.EqualFold(x, fileType) {
							allowed = true
							break
						}
					}
				} else {
					allowed = true
				}
				if !allowed {
					return nil, errors.New("the uploaded file type is not permitted")
				}

				_, err = inFile.Seek(0, 0)
				if err != nil {
					return nil, err
				}
				var uploadedFile UploadedFile
				if renameFile {
					uploadedFile.NewFileName = fmt.Sprintf("%s%s", t.RandomString(25), filepath.Ext(hdrs.Filename))
				} else {
					uploadedFile.NewFileName = hdrs.Filename
				}
				uploadedFile.OriginalFileName = hdrs.Filename

				if outFile, err := os.Create(filepath.Join(uploadDir, uploadedFile.NewFileName)); err != nil {
					return nil, err
				} else {
					fileSize, err := io.Copy(outFile, inFile)
					if err != nil {
						return nil, err
					}
					uploadedFile.FileSize = fileSize
				}
				uploadedFiles = append(uploadedFiles, &uploadedFile)
				return uploadedFiles, nil

			} (upLoadedFiles)
			if err != nil {
				return upLoadedFiles, err
			}
		}
	}

	return upLoadedFiles, nil
}