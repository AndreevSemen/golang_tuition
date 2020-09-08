package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Filesystem point interface
type FSPoint interface {
	PointName() string
}

type Directory struct {
	Name    string
	Insides []FSPoint
}

type File struct {
	Name string
	Size int64
}

func (d Directory) PointName() string {
	return d.Name
}

func (f File) PointName() string {
	if f.Size < 0 {
		panic(fmt.Sprintf("file %s with negative size: %d\n", f.Name, f.Size))
	} else if f.Size == 0 {
		return fmt.Sprintf("%s (empty)", f.Name)
	} else {
		return fmt.Sprintf("%s (%db)", f.Name, f.Size)
	}
}

func InspectDir(path string, printFiles bool) ([]FSPoint, error) {
	// Checking if path is a path to the directory
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	} else if !fileInfo.IsDir() {
		return nil, errors.New("only directories could be inspected")
	}

	// Reading a directory
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	files, err := file.Readdir(0)
	if err != nil {
		return nil, err
	} else if err := file.Close(); err != nil {
		return nil, err
	}

	// Sorting a file slice
	sort.Slice(files, func(i int, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	// Inspecting a directory
	var points []FSPoint
	for _, pointInfo := range files {
		if pointInfo.IsDir() {
			insides, err := InspectDir(filepath.Join(path, pointInfo.Name()), printFiles)
			if err != nil {
				return nil, err
			}

			points = append(points,
				Directory{
					Name:    pointInfo.Name(),
					Insides: insides,
				},
			)
		} else if printFiles {
			points = append(points,
				File{
					Name: pointInfo.Name(),
					Size: pointInfo.Size(),
				},
			)
		}
	}

	return points, nil
}

func StringifyPoints(points []FSPoint, prefix string) string {
	var s = ""
	for i := range points {
		var headPrefix, bodyPrefix string
		if i == len(points)-1 {
			headPrefix = "└───"
			bodyPrefix = "\t"

		} else {
			headPrefix = "├───"
			bodyPrefix = "│\t"
		}

		s += prefix + headPrefix + points[i].PointName() + "\n"

		if dir, ok := points[i].(Directory); ok {
			s += StringifyPoints(dir.Insides, prefix+bodyPrefix)
		}
	}
	return s
}

func dirTree(out io.Writer, path string, printFiles bool) error {
	points, err := InspectDir(path, printFiles)
	if err != nil {
		return err
	}

	if _, err := out.Write([]byte(StringifyPoints(points, ""))); err != nil {
		return err
	}

	return nil
}

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}
