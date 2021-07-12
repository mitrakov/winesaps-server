package filereader

import "os"
import "fmt"
import "strings"
import "io/ioutil"
import "math/rand"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// FileReader is a component that reads files from disk and stores them in memory as a bytearray.
// This component is independent.
type FileReader struct {
    files map[string][]byte // no lock required (immutable map)
}

// NewFileReader creates a new FileReader. Please do not create a FileReader directly.
// "path" - a directory to read all files from, filtered by "extension" ("path" may be relative).
// "bufSiz" - maximum size of a file, in bytes (if this value is too small, a bytearray can be truncated)
// Example: NewFileReader("descriptions/texts", "txt", 2048)
func NewFileReader(path, extension string, bufSiz uint) (*FileReader, *Error) {
    res := &FileReader{make(map[string][]byte)}
    
    // load all the level files to memory
    files, err := ioutil.ReadDir(path)
    if err == nil {
        for _, f := range files {
            if strings.HasSuffix(f.Name(), extension) {
                data, err := res.readFile(fmt.Sprintf("%s/%s", path, f.Name()), bufSiz)
                if err == nil {
                    res.files[f.Name()] = data
                } else {
                    return res, err
                }
            }
        }
    }
    return res, NewErrFromError(res, 20, err)
}

// GetByName returns file content by a given file name
func (reader *FileReader) GetByName(name string) (res []byte, ok bool) {
    res, ok = reader.files[name]
    return
}

// GetRandomExcept returns a random file name from the list of loaded files, except specified as "excepts" parameter.
func (reader *FileReader) GetRandomExcept(excepts ...string) (string, *Error) {
    exceptMap := make(map[string]bool)
    for _, except := range excepts {
        exceptMap[except] = true
    }
    n := len(reader.files) - len(exceptMap)

    if n > 0 {
        r := rand.Intn(n)
        i := 0
        for name := range reader.files {
            if _, ok := exceptMap[name]; !ok {
                if i == r {
                    return name, nil
                }
                i++
            }
        }
    }
    return "", NewErr(reader, 21, "No levels loaded")
}

// GetRandomAmong returns a random file name from the list of loaded files, that also is present in "among" parameter.
func (reader *FileReader) GetRandomAmong(among ...string) (string, *Error) {
    if len(among) > 0 {
        r := rand.Intn(len(among))
        res := among[r]
        if _, ok := reader.files[res]; ok {
            return res, nil
        }
        return "", NewErr(reader, 22, "File not found", res)
    }
    return "", NewErr(reader, 23, "Incorrect arg length")
}

// readFile is an internal function to read a file with a given file name as a bytearray.
// "filename" - file name
// "bufSiz" - maximum size of a file, in bytes (if this value is too small, a bytearray can be truncated)
func (reader *FileReader) readFile(filename string, bufSiz uint) ([]byte, *Error) {
    f, err := os.Open(filename)
    if err == nil {
        data := make([]byte, bufSiz)
        var n int
        n, err = f.Read(data)
        if err == nil {
            err = f.Close()
            if err == nil {
                return data[:n], nil
            }
        }
    }
    return []byte{}, NewErrFromError(reader, 24, err)
}
