package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nfnt/resize"
	"golang.org/x/text/encoding/charmap"
)

var wg sync.WaitGroup

func main() {

	DESTROOT := filepath.Join("Z:", "Supervisora", "RTA")
	// DESTROOT = "D:\\teste"
	ORIGINROOT := filepath.Join("D:\\Documentos", "Users", "Eduardo", "Documentos", "ANTT", "OneDrive - ANTT- Agencia Nacional de Transportes Terrestres", "CRO", "Relatórios RTA")

	fmt.Println(DESTROOT)

	files := getFilesFromDirectory(ORIGINROOT)

	fmt.Println("ESCOLHA: ")
	for i, file := range files {
		fmt.Printf("%d -- > %s \n", i+1, file.Name())
	}

	var line string

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	line = scanner.Text()

	option := parseToInt(line)

	for option < 1 || option > len(files) {
		fmt.Println("ESCOLHA: ")
		for i, fileName := range files {
			fmt.Printf("%d -- > %s \n", i, fileName.Name())
		}
		scanner.Scan()
		line = scanner.Text()

		option = parseToInt(line)
	}

	chosenFile := files[option-1]
	chosenFilename := chosenFile.Name()
	var tipoRelatorio string

	if strings.Contains(strings.ToLower(chosenFilename), "diário") {
		tipoRelatorio = "Relatórios diários"
	} else if strings.Contains(strings.ToLower(chosenFilename), "acompanhamento") {
		tipoRelatorio = "Relatórios Semanais"
	} else if strings.Contains(strings.ToLower(chosenFilename), "obra") {
		tipoRelatorio = "Relatórios de Obras"
	} else {
		tipoRelatorio = "Relatórios Diversos"
	}

	chosenPath := filepath.Join(ORIGINROOT, chosenFilename)

	yearMonth, year := getCreationTimeWindows(chosenPath)

	destDirectory := filepath.Join(DESTROOT, tipoRelatorio, year, yearMonth)

	if chosenFile.IsDir() {
		e := os.MkdirAll(destDirectory, os.ModeAppend.Perm())
		if e != nil {
			fmt.Println(e)
		}

	}

	zipFiles := copyNotZipFilesAndReturnZips(chosenPath, destDirectory)

	for _, zipFile := range zipFiles {
		if strings.HasSuffix(zipFile, "unzipped") {
			continue
		}
		unzipedFileFolder, e := Unzip(zipFile, destDirectory, true)
		if e != nil {
			fmt.Println(e)
		}

		filesInFolderInfo, e := ioutil.ReadDir(unzipedFileFolder)
		if e != nil {
			fmt.Println(e)
		}

		for _, fileInfo := range filesInFolderInfo {
			if fileInfo.IsDir() {

				// ZipWriter(filepath.Join(unzipedFileFolder, fileInfo.Name()), filepath.Join(unzipedFileFolder, fileInfo.Name()))

				zipSource := filepath.Join(unzipedFileFolder, fileInfo.Name())
				zipitTarget := filepath.Join(filepath.Dir(unzipedFileFolder), fileInfo.Name()) + "_compressed.zip"

				e = zipit(zipSource, zipitTarget)
				if e != nil {
					fmt.Println(e)
				}
				wg.Add(1)

				destinationFile := filepath.Join(destDirectory, filepath.Base(filepath.Dir(filepath.Dir(zipitTarget))), filepath.Base(filepath.Dir(zipitTarget)), filepath.Base(zipitTarget))
				fmt.Println(destinationFile)
				go copyFiles(zipitTarget, destinationFile)

			}
		}

	}

	wg.Wait()
}

func getCreationTimeWindows(src string) (monthYear string, year string) {
	info, e := os.Stat(src)
	if e != nil {
		fmt.Println(e)
	}
	winAtt := info.Sys().(*syscall.Win32FileAttributeData)
	nano := winAtt.CreationTime.Nanoseconds()

	date := time.Unix(0, nano)
	monthYear = fmt.Sprint(date.Format("2006-01"))
	year = fmt.Sprint(date.Year())
	return
}

func parseToInt(str string) int {
	parsed, e := strconv.Atoi(str)
	if e != nil {
		fmt.Println(e)
		return -1
	}
	return parsed
}
func getFilesFromDirectory(directory string) []os.FileInfo {

	outputDirRead, e := os.Open(directory)
	if e != nil {
		fmt.Println(e)
		fmt.Println("linha 206")
	}

	// Call Readdir to get all files.
	outputDirFiles, e := outputDirRead.Readdir(0)

	if e != nil {
		fmt.Println(e)
	}

	var list []string
	// Loop over files.
	for outputIndex := range outputDirFiles {
		outputFileHere := outputDirFiles[outputIndex]

		// Get name of file.
		outputNameHere := outputFileHere.Name()

		list = append(list, outputNameHere)
	}
	return outputDirFiles
}

func copyNotZipFilesAndReturnZips(src string, dest string) []string {
	var absPath string

	var zipFiles []string

	var fun func(src string)

	fun = func(src string) {

		fileInfo, _ := ioutil.ReadDir(src)

		for _, file := range fileInfo {

			absPath = filepath.Join(src, file.Name())
			destNow := filepath.Join(dest, absPath[124:])

			if !file.IsDir() {
				if filepath.Ext(absPath) == ".zip" {
					zipFiles = append(zipFiles, absPath)

					continue
				}
				if strings.HasSuffix(absPath, ".docx") || strings.HasSuffix(absPath, ".pdf") {

					wg.Add(1)
					go copyFiles(absPath, destNow)

				}
			}

			if file.IsDir() {
				fmt.Println("inicio subDiretorio :")
				fun(absPath)
			}

		}
	}

	fun(src)

	return zipFiles
}

func copyFiles(src, dest string) {

	srcFile, e := os.Open(src)
	if e != nil {
		fmt.Println(e)
	}

	_, e = os.Stat(dest)
	if os.IsNotExist(e) {
		os.MkdirAll(filepath.Dir(dest), os.ModePerm)
	}

	destFile, e := os.Create(dest)
	if e != nil {
		fmt.Println(e)
	}

	_, e = io.Copy(destFile, srcFile)
	if e != nil {
		fmt.Printf(" ****  Não foi possível copiar o arquivo %s ***** \n", srcFile.Name())
		fmt.Println(e)
	}
	destFile.Close()
	srcFile.Close()
	wg.Done()

}

func Unzip(src string, dest string, resize bool) (string, error) {

	var filenames []string

	x := filepath.Dir(src)

	if strings.HasSuffix(dest, "unzipped") {
		return "", nil
	}

	dest = filepath.Join(x, "unzipped")

	zipReader, err := zip.OpenReader(src)
	if err != nil {
		return "", err
	}
	defer zipReader.Close()

	var fpath string

	for _, f := range zipReader.File {

		// name, _ := charmap.CodePage850.NewDecoder().String(zipReader.File[i].Name)

		name, _ := charmap.CodePage850.NewDecoder().String(f.Name)

		// Store filename/path for returning and using later on
		fpath = filepath.Join(dest, name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return "", fmt.Errorf("%s: illegal file path", fpath)
		}

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		filenames = append(filenames, fpath)

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return "", err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return "", err
		}

		rc, err := f.Open()
		if err != nil {
			return "", err
		}

		if resize {

			resizeImage(rc, outFile)
		} else {

			_, err = io.Copy(outFile, rc)

		}

		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return "", err
		}
	}

	for _, v := range filenames {
		fmt.Println(v)
	}

	return dest, nil
}

func zipit(source, target string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})

	return err
}

func resizeImage(rcP io.Reader, outFile *os.File) {
	// decode jpeg into image.Image
	rc := rcP
	img, err := jpeg.Decode(rc)
	if err != nil {
		log.Fatal(err)
	}

	// resize to width 1000 using Lanczos resampling
	// and preserve aspect ratio
	m := resize.Resize(500, 0, img, resize.Bicubic)

	jpeg.Encode(outFile, m, nil)

	// _, err = io.Copy(outFile, rc)

	// Close the file without defer to close before next iteration of loop
	outFile.Close()

}
