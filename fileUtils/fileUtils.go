package fileUtils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"math"
	"os"
	"strings"

	"github.com/nfnt/resize"
)

func GetFileType(filename string, extension string, isFolder bool) string {
	// Edge cases:
	switch strings.ToLower(filename) {
	case "desktop":
		return "folderDesktop"
	case "downloads":
		return "folderDownloads"
	case "documents":
		return "folderDocuments"
	case "pictures":
		return "folderPictures"
	case "music":
		return "folderMusic"
	}
	if isFolder {
		return "folder"
	}

	ftype := fileTypeMap[extension]
	if ftype != "" {
		return ftype
	}
	return "file"
}

func IsHidden(fpath string) bool {
	return false
}
func CreatedAt(fpath string) int {
	return 0
}
func ModifiedAt(fpath string) int {
	return 0
}

var fileTypeMap map[string]string

func LoadJSON() {
	jsonUbi := "./extensionData.json"

	jsonContent, err := ioutil.ReadFile(jsonUbi)
	if err != nil {
		fmt.Printf("Error reading file type json:%s\n", err)
		os.Exit(1)
	}

	err = json.Unmarshal([]byte(jsonContent), &fileTypeMap)
	if err != nil {
		fmt.Printf("Error reading file type json:%s\n", err)
		os.Exit(1)
	}
	fmt.Println("👍 File type JSON loaded")
}

func GetImagePreview(fpath string, extension string) string {
	content, err := os.Open(fpath)
	if err != nil {
		fmt.Printf("Error opening the image '%s'", fpath)
		return ""
	}
	defer content.Close()

	imageDecoded, _, err := image.Decode(content)
	if err != nil {
		fmt.Printf("Error decoding the image '%s'", fpath)
		return ""
	}

	maxSize := 90.0

	width := float64(imageDecoded.Bounds().Dx())
	height := float64(imageDecoded.Bounds().Dy())
	aspectRatio := width / height

	var newWidth, newHeight uint
	if aspectRatio > 1 {
		newWidth = uint(maxSize)
		newHeight = uint(math.Round(float64(newWidth) / aspectRatio))
	} else {
		newHeight = uint(maxSize)
		newWidth = uint(math.Round(float64(newHeight) * aspectRatio))
	}

	// Resize the input image to create a preview
	previewImage := resize.Resize(uint(newWidth), uint(newHeight), imageDecoded, resize.Lanczos3)

	var buffer bytes.Buffer

	if extension == ".png" {
		err = png.Encode(&buffer, previewImage)
		if err != nil {
			fmt.Println("Error encoding preview image as PNG:", err)
			return ""
		}
	} else if extension == ".jpg" || extension == ".jpeg" {
		err = jpeg.Encode(&buffer, previewImage, nil)
		if err != nil {
			fmt.Println("Error encoding preview image as JPG:", err)
			return ""
		}
	}

	previewBytes := buffer.Bytes()
	previewStringB64 := base64.StdEncoding.EncodeToString(previewBytes)
	return previewStringB64
}
