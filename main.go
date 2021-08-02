package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	fileMode   = 0644
	fileCreate = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	byteUp     = 'a' - 'A'

	nameFlag = "name"
	dirFlag  = "dir"
	onceFlag = "once"

	pubConsts   = "pub_consts.h"
	pubTypes    = "pub_types.h"
	pubInlines  = "pub_inlines.h"
	pubIncludes = "pub_includes.h"

	privConsts   = "priv_consts.h"
	privTypes    = "priv_types.h"
	privInlines  = "priv_inlines.h"
	privIncludes = "priv_includes.h"
)

var (
	name   string
	useDir bool
	once   bool

	isNameSet bool
	isDirSet  bool
)

func init() {
	flag.StringVar(&name, nameFlag, "", "module (parent dir) name")
	flag.BoolVar(&useDir, dirFlag, false, "ude directory name as name")
	flag.BoolVar(&once, onceFlag, false, "add #pragma once to includes")
}

func checkFlags() {
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case nameFlag:
			isNameSet = true
		case dirFlag:
			isDirSet = true
		}
	})
	if isNameSet && isDirSet {
		fmt.Fprintf(os.Stderr, "flags -%s and -%s not compatible\n", nameFlag, dirFlag)
		flag.Usage()
		os.Exit(1)
	}
	if !isDirSet {
		return
	}
	path, err := os.Getwd()
	if err != nil {
		logErr(err)
	}
	name = filepath.Base(path)
}

func main() {
	checkFlags()
	if err := createNamedInc(pubConsts); err != nil {
		logErr(err)
	}
	if err := createNamedInc(pubTypes, pubConsts); err != nil {
		logErr(err)
	}
	if err := createNamedInc(pubInlines, pubTypes); err != nil {
		logErr(err)
	}
	if err := createNamedInc(pubIncludes, pubInlines); err != nil {
		logErr(err)
	}
	if err := createNamedInc(privConsts, pubIncludes); err != nil {
		logErr(err)
	}
	if err := createNamedInc(privTypes, privConsts); err != nil {
		logErr(err)
	}
	if err := createNamedInc(privInlines, privTypes); err != nil {
		logErr(err)
	}
	if err := createNamedInc(privIncludes, privInlines); err != nil {
		logErr(err)
	}
	if len(name) == 0 {
		return
	}
	if err := createNamedInc(name+".h", pubIncludes); err != nil { // export include
		logErr(err)
	}
	if err := createNamedSrc(name+".c", privIncludes); err != nil { // src template
		logErr(err)
	}
}

func logErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
	}
	os.Exit(1)
}

func defName(iname string) string {
	n := includeName(iname)
	r := make([]byte, 0, len(n)+8)
	r = append(r, '_', '_')
	for _, b := range []byte(n) {
		switch {
		case b >= 'A' && b <= 'Z':
			r = append(r, b)
		case b >= 'a' && b <= 'z':
			r = append(r, b-byteUp)
		default:
			r = append(r, '_')
		}
	}
	r = append(r, '_', '_')
	return string(r)
}

func includeName(iname string) string {
	if len(name) == 0 {
		return iname
	}
	return name + "/" + iname
}

func writeHeader(w io.Writer, iname string) (err error) {
	if once {
		if _, err = fmt.Fprintf(w, "#pragma once\n\n"); err != nil {
			return
		}
	}
	def := defName(iname)
	_, err = fmt.Fprintf(w, "#ifndef %s\n#define %s\n\n", def, def)
	return
}

func writeFooter(w io.Writer, iname string) (err error) {
	_, err = fmt.Fprintf(w, "#endif //%s\n", defName(iname))
	return
}

func writeInclude(w io.Writer, iname string) (err error) {
	_, err = fmt.Fprintf(w, "#include \"%s\"\n", includeName(iname))
	return
}

func createFile(fname string) (io.WriteCloser, string, error) {
	fn, err := filepath.Abs(strings.ToLower(fname))
	if err != nil {
		return nil, "", err
	}
	fd, err := os.OpenFile(fn, fileCreate, fileMode)
	if err != nil {
		return nil, "", err
	}
	return fd, fn, nil
}

func createNamedInc(iname string, inc ...string) (err error) {
	var (
		w  io.WriteCloser
		fn string
	)
	if w, fn, err = createFile(iname); err != nil {
		return
	}
	defer func() {
		w.Close()
		if err != nil {
			os.Remove(fn)
		}
	}()
	if err = writeHeader(w, iname); err != nil {
		return
	}
	for _, incName := range inc {
		if err = writeInclude(w, incName); err != nil {
			return
		}
	}
	if len(inc) > 0 {
		if _, err = w.Write([]byte{'\n'}); err != nil {
			return
		}
	}
	if err = writeFooter(w, iname); err != nil {
		return
	}
	return
}

func createNamedSrc(sname string, inc ...string) (err error) {
	var (
		w  io.WriteCloser
		fn string
	)
	if w, fn, err = createFile(sname); err != nil {
		return
	}
	defer func() {
		w.Close()
		if err != nil {
			os.Remove(fn)
		}
	}()
	for _, incName := range inc {
		if err = writeInclude(w, incName); err != nil {
			return
		}
	}
	return
}
