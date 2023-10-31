package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gyozora/data"
	"gyozora/data/appcache"
	"gyozora/fileUtils"
	"gyozora/models"
	"gyozora/sysUtils"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"
)

var CURRENT_PATH = ""

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetUserOS() string {
	return runtime.GOOS
}

func (a *App) ReadPath(currentpath string, path string) models.ReadPathResponse {
	CURRENT_PATH = path
	dirFiles := make([]models.SysFile, 0)
	dirFolders := make([]models.SysFile, 0)
	breadcrumbs := make([]models.SysFile, 0)

	files, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return models.ReadPathResponse{Error: models.SimpleError{Status: true, Reason: "Folder not found."}}
		} else if errors.Is(err, os.ErrPermission) {
			return models.ReadPathResponse{Error: models.SimpleError{Status: true, Reason: "Access denied."}}
		}
		return models.ReadPathResponse{Error: models.SimpleError{Status: true, Reason: "Unexpected error."}}
	}

	// Load path content
	for _, file := range files {
		generatedSysFile, err := fileUtils.GenerateSysFile(currentpath, filepath.Join(path, file.Name()))
		if err == nil {
			if file.IsDir() {
				dirFolders = append(dirFolders, generatedSysFile)
			} else {
				dirFiles = append(dirFiles, generatedSysFile)
			}
		}
	}

	// Load path as breadcrumbs
	for _, folderPath := range GetPathFolders(path) {
		generatedBreadcrumb, err := fileUtils.GenerateSysFile(currentpath, folderPath)
		if err == nil {
			breadcrumbs = append(breadcrumbs, generatedBreadcrumb)
		}
	}

	return models.ReadPathResponse{DirFiles: dirFiles, DirFolders: dirFolders, Breadcrumbs: breadcrumbs, Error: models.SimpleError{Status: false}}
}

func GetPathFolders(fpath string) []string {
	var pathes []string

	previousPath := ""
	curPath := fpath
	for curPath != "" && curPath != previousPath {
		pathes = append(pathes, curPath)
		previousPath = curPath
		curPath = filepath.Dir(curPath)
	}

	return pathes
}

func (a *App) LoadPinnedFolders() []models.LeftBarElement {
	pinnedFolders := make([]models.LeftBarElement, 0)

	if runtime.GOOS == "windows" {
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Desktop", Type: "folderDesktop", Path: filepath.Join(sysUtils.UserHomedir(), "Desktop")})
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Downloads", Type: "folderDownloads", Path: filepath.Join(sysUtils.UserHomedir(), "Downloads")})
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Documents", Type: "folderDocuments", Path: filepath.Join(sysUtils.UserHomedir(), "Documents")})
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Pictures", Type: "folderPictures", Path: filepath.Join(sysUtils.UserHomedir(), "Pictures")})
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Music", Type: "folderMusic", Path: filepath.Join(sysUtils.UserHomedir(), "Music")})
	} else if runtime.GOOS == "darwin" {
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Applications", Type: "folderApplications", Path: filepath.Join(sysUtils.UserRoots()[0], "Applications")})
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Desktop", Type: "folderDesktop", Path: filepath.Join(sysUtils.UserHomedir(), "Desktop")})
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Documents", Type: "folderDocuments", Path: filepath.Join(sysUtils.UserHomedir(), "Documents")})
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Downloads", Type: "folderDownloads", Path: filepath.Join(sysUtils.UserHomedir(), "Downloads")})
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Pictures", Type: "folderPictures", Path: filepath.Join(sysUtils.UserHomedir(), "Pictures")})
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Music", Type: "folderMusic", Path: filepath.Join(sysUtils.UserHomedir(), "Music")})
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: "Movies", Type: "folderMovies", Path: filepath.Join(sysUtils.UserHomedir(), "Movies")})
	}

	return pinnedFolders
}

func (a *App) LoadYourComputer() []models.LeftBarElement {
	userRoots := sysUtils.UserRoots()
	pinnedFolders := make([]models.LeftBarElement, 0)
	for i := 0; i < len(userRoots); i++ {
		pinnedFolders = append(pinnedFolders, models.LeftBarElement{Name: fmt.Sprintf("Disk %d (%s)", i, userRoots[i]), Type: "folderDisk", Path: userRoots[i]})
	}

	return pinnedFolders
}

func (a *App) GetStartingPath() string {
	CURRENT_PATH = sysUtils.UserHomedir()
	return filepath.Join(CURRENT_PATH, "Desktop")
}

var ACTIVE_JOBS = -1

func (a *App) RenderPreview(file models.SysFile, unixBeginning int, remaining int, useCache bool) models.SysFile {

	if ACTIVE_JOBS != -1 { // There is a currently active JOB
		if ACTIVE_JOBS != unixBeginning { // If it's not our job, cancel the other job
			ACTIVE_JOBS = unixBeginning
		}
	}

	ACTIVE_JOBS = unixBeginning

	if ACTIVE_JOBS != unixBeginning { // We were cancelled
		return file
	}

	if file.IconClass != "fileImage" {
		if remaining == 0 {
			ACTIVE_JOBS = -1
		}
		return file
	}

	imageIsCached := true
	if useCache {
		imageIsLatest := false
		//Check on DB if exists:
		b64img, imageIsLatest, err := appcache.GetCachedPreview(file)
		if err != nil {
			imageIsCached = false
		}

		if imageIsCached && imageIsLatest {
			file.Preview = b64img
			return file
		}
	}

	// If we get here, image is not cached or is not cached to the latest version

	generatedPreview := fileUtils.GetImagePreview(file.PathFull, file.Extension)
	file.Preview = generatedPreview

	if ACTIVE_JOBS != unixBeginning {
		return file
	}

	// Create or update preview in cache
	if useCache {
		appcache.AddOrUpdatePreview(file, generatedPreview)
	}

	if ACTIVE_JOBS != unixBeginning {
		return file
	}

	// If we were not cancelled
	if remaining == 0 {
		ACTIVE_JOBS = -1
	}

	return file
}

// File(s) actions
func (a *App) OpenFile(fpath string) models.ActionResponse {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", fpath)
	case "darwin":
		cmd = exec.Command("open", fpath)
	case "linux":
		cmd = exec.Command("xdg-open", fpath) // TODO: Test
	default:
		return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Unsupported operating system."}}
	}

	fmt.Printf("Trying to execute the file with '%s'\n", cmd)
	err := cmd.Run()
	if err != nil {
		return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Access denied."}}
	}

	return models.ActionResponse{Error: models.SimpleError{Status: false}}
}

func (a *App) AddFile(path string, filename string, fileType string) models.ActionResponse {
	finalPath := filepath.Join(path, filename)

	if err := fileUtils.Exists(finalPath); err == nil {
		return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "File already exists."}}
	}

	var file *os.File
	var err error
	if fileType == "folder" {
		err = os.Mkdir(finalPath, 0755)
	} else {
		file, err = os.Create(finalPath)
	}

	if err != nil {
		valid := sysUtils.IsFilenameValid(filename)

		if !valid { // Uses an illegal character for a filename in that OS
			return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: fmt.Sprintf("A filename can't contain %s.", sysUtils.GetInvalidFilenameCharacters())}}
		} else {
			return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Unexpected error while creating the new file."}}
		}
	}
	file.Close()

	createdFile, err := fileUtils.GenerateSysFile(path, filepath.Join(path, filename))
	if err != nil {
		return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Error getting the information of the created file."}}
	}

	return models.ActionResponse{Error: models.SimpleError{Status: false}, File: createdFile}
}

func (a *App) CutFile_s(fpaths []string) {
	fmt.Println("TBD -1")
}

// TODO: Duplicating folders
func (a *App) PasteFolder(srcFolder models.SysFile, tgtPath string, isBase bool) models.PasteFileResponse {
	srcPath := srcFolder.PathFull

	tgtPathFolder := filepath.Join(tgtPath, srcFolder.Filename)
	if !isBase {
		relPath := strings.Replace(srcFolder.PathRelativeFull, "..", "", 1)
		relPath = strings.Replace(relPath, string(filepath.Separator), "", 1)

		tgtPathFolder = filepath.Dir(filepath.Join(tgtPath, relPath, srcFolder.Filename))
	}

	if err := fileUtils.Exists(srcPath); err != nil {
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: "File not found."}}
	}

	if err := fileUtils.Exists(tgtPath); err != nil {
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: "Target path not found."}}
	}

	if err := fileUtils.Exists(tgtPathFolder); err == nil { // Folder exists on paste location
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: "Folder already exists"}} // TODO: Check if same dir, allow - Copy
	}

	if err := os.Mkdir(tgtPathFolder, 0755); err != nil {
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: err.Error()}}
	}

	pastedFile, err := fileUtils.GenerateSysFile(filepath.Dir(tgtPathFolder), tgtPathFolder)
	if err != nil {
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: "Error getting the information of the pasted folder."}}
	}
	pastedFile.Preview = srcFolder.Preview

	return models.PasteFileResponse{File: pastedFile, Error: models.SimpleError{Status: false}}
}

func (a *App) PasteFile(srcFile models.SysFile, tgtPath string, isBase bool) models.PasteFileResponse {
	srcPath := srcFile.PathFull

	tgtPathParentFolder := filepath.Join(tgtPath)
	if !isBase {
		relPath := strings.Replace(srcFile.PathRelativeFull, "..", "", 1)
		relPath = strings.Replace(relPath, string(filepath.Separator), "", 1)

		tgtPathParentFolder = filepath.Dir(filepath.Join(tgtPath, relPath))
	}

	if err := fileUtils.Exists(srcPath); err != nil {
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: "File not found."}}
	}

	if err := fileUtils.Exists(tgtPath); err != nil {
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: "Target path not found."}}
	}

	//Check if pasting location exist a file with that filename
	tgtPathFile := filepath.Join(tgtPathParentFolder, srcFile.Filename)
	fileExists := false

	if err := fileUtils.Exists(tgtPathFile); err == nil {
		fileExists = true
	}

	if fileExists && tgtPath == srcFile.Path { // If we are in the same path, allow, for file duplicate creation
		index := 0

		fileExists = true
		for fileExists {
			if index == 0 {
				tgtPathFile = filepath.Join(tgtPathParentFolder, fmt.Sprintf("%s - Copy%s", srcFile.Name, srcFile.Extension))
			} else {
				tgtPathFile = filepath.Join(tgtPathParentFolder, fmt.Sprintf("%s - Copy (%d)%s", srcFile.Name, index, srcFile.Extension))
			}

			if err := fileUtils.Exists(tgtPathFile); err == nil {
				fileExists = true
				index++
			} else {
				fileExists = false
			}
		}
	} else if fileExists {
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: "File already exists"}}
	}

	content, err := os.ReadFile(srcPath)
	if err != nil {
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: "Could not read file."}}
	}

	if err := os.WriteFile(tgtPathFile, content, 0755); err != nil {
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: "Error writing the file."}}
	}

	pastedFile, err := fileUtils.GenerateSysFile(tgtPathParentFolder, tgtPathFile)
	if err != nil {
		return models.PasteFileResponse{Error: models.SimpleError{Status: true, Reason: "Error getting the information of the pasted file."}}
	}
	pastedFile.Preview = srcFile.Preview

	if pastedFile.Preview != "" {
		appcache.AddOrUpdatePreview(pastedFile, pastedFile.Preview)
	}

	return models.PasteFileResponse{File: pastedFile, Error: models.SimpleError{Status: false}}
}

func (a *App) RenameFile(file models.SysFile, newFilename string) models.ActionResponse { // TODO: Update cache DB
	newPath := filepath.Join(file.Path, newFilename)

	if err := fileUtils.Exists(newPath); err == nil {
		return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "File already exists."}}
	}

	err := os.Rename(file.PathFull, newPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Folder not found."}}
		} else if errors.Is(err, os.ErrPermission) {
			return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Access denied."}}
		}
		return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Unexpected error."}}
	}

	renamedFile, err := fileUtils.GenerateSysFile(file.Path, filepath.Join(file.Path, newFilename)) // Generate a new sys file (to handle renaming to different extension)
	if err != nil {
		return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Error getting the information of the renamed file."}}
	}

	if file.Extension == renamedFile.Extension {
		renamedFile.Preview = file.Preview
	}

	appcache.MovePreview(file, renamedFile)

	return models.ActionResponse{Error: models.SimpleError{Status: false}, File: renamedFile}
}

func (a *App) DeleteFile(file models.SysFile) models.SimpleError {
	if err := os.RemoveAll(file.PathFull); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return models.SimpleError{Status: true, Reason: "Access denied."}
		}
		return models.SimpleError{Status: true, Reason: "Unexpected error."}
	}

	if file.Preview != "" {
		appcache.DeletePreview(file)
	}

	if file.IsFolder {
		appcache.DeletePreviewsInside(file)
	}

	return models.SimpleError{Status: false}
}

func (a *App) PropertiesFile(fpath string) models.ActionResponse {
	if err := fileUtils.Exists(fpath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "File not found."}}
		} else if errors.Is(err, os.ErrPermission) {
			return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Access denied."}}
		}
		return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Unexpected error."}}
	}

	generatedSysFile, err := fileUtils.GenerateSysFile(fpath, fpath)
	if err != nil {
		return models.ActionResponse{Error: models.SimpleError{Status: true, Reason: "Error getting the information of the pasted file."}}
	}
	return models.ActionResponse{File: generatedSysFile, Error: models.SimpleError{Status: false}}
}

func (a *App) FileExists(fpath string) bool {
	if err := fileUtils.Exists(fpath); err == nil {
		return true
	}
	return false
}

// Settings:

func (a *App) Go_LoadSettings() map[string]string {
	var configs []models.Config

	data.DataDB.Select(&configs, "SELECT * FROM config")

	fmt.Println("⚙️ Loaded settings:")
	fmt.Println(configs)

	settings := make(map[string]string)
	for _, config := range configs {
		settings[config.Name] = config.Value
	}
	return settings
}

func (a *App) Go_GetSetting(name string) string {
	value := "null"
	data.DataDB.QueryRowx("SELECT value FROM config WHERE name=?", name).Scan(&value)
	return value
}

func (a *App) Go_SetSetting(name string, value string) models.SimpleError {
	_, err := data.DataDB.Query("INSERT OR REPLACE INTO config(name, value) VALUES(?, ?)", name, value)
	if err != nil {
		return models.SimpleError{Status: true}
	}
	return models.SimpleError{Status: false}
}

func (a *App) Go_DeleteSetting(name string) models.SimpleError {
	_, err := data.DataDB.Query("DELETE FROM config WHERE name=?", name)
	if err != nil {
		return models.SimpleError{Status: true}
	}
	return models.SimpleError{Status: false}
}

func (a *App) Go_CacheSize() string {
	cache := make([]models.CachePreview, 0)
	data.DataDB.Select(&cache, "SELECT * FROM cache")

	sizeB := 0
	for _, cacheElem := range cache {
		sizeB += len(cacheElem.PathFull)
		sizeB += int(unsafe.Sizeof(cacheElem.DateModification))
		sizeB += len(cacheElem.Preview)
	}

	if sizeB == 0 {
		return "0B"
	}

	sizeKB := float64(sizeB) / 1024.0

	return fmt.Sprintf("%.2fKB", sizeKB)
}

func (a *App) Go_CacheClear() models.SimpleError {
	_, err := data.DataDB.Query("DELETE FROM cache")
	if err != nil {
		return models.SimpleError{Status: true}
	}
	return models.SimpleError{Status: false}
}

func (a *App) Go_LoadDictionary() map[string]map[string]string {
	finalDictionary := make(map[string]map[string]string)

	finalDictionary["EN"] = DictionaryData("EN")
	finalDictionary["ES"] = DictionaryData("ES")

	return finalDictionary
}

func DictionaryData(lang string) map[string]string {
	langDict := make(map[string]string)

	fileEN, err := os.ReadFile(fmt.Sprintf("%s.json", lang))
	if err != nil {
		fmt.Println("Error reading file EN.json")
		return langDict
	}

	err = json.Unmarshal(fileEN, &langDict)
	return langDict
}
