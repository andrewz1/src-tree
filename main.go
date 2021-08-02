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
	fileCreate = os.O_WRONLY | os.O_CREATE | os.O_EXCL | os.O_TRUNC
	byteUp     = 'a' - 'A'

	nameFlag = "name"
	dirFlag  = "dir"
	onceFlag = "once"
	pubFlag  = "pub"
	addFlag  = "add"

	sConsts  = "_consts.h"
	sTypes   = "_types.h"
	sInlines = "_inlines.h"
	pPub     = "pub"
	pPriv    = "priv"
)

var (
	name   string // module name
	useDir bool   // current dir is module name
	once   bool   // add #pragma once
	pub    bool   // generate only public tree
	pfx    string // prefix for files

	isNameSet bool
	isDirSet  bool
	isPfxSet  bool
)

func init() {
	flag.StringVar(&name, nameFlag, "", "module (parent dir) name")
	flag.BoolVar(&useDir, dirFlag, false, "ude directory name as name")
	flag.BoolVar(&once, onceFlag, false, "add #pragma once to includes")
	flag.BoolVar(&pub, pubFlag, false, "generate only public part of tree")
	flag.StringVar(&pfx, addFlag, "", "add part of tree with given prefix (for example xxx give the xxx_consts.h and so on)")
}

func checkFlags() {
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case nameFlag:
			isNameSet = true
		case dirFlag:
			isDirSet = true
		case addFlag:
			isPfxSet = true
		}
	})
	if isNameSet && isDirSet {
		fmt.Fprintf(os.Stderr, "flags -%s and -%s not compatible\n", nameFlag, dirFlag)
		flag.Usage()
		os.Exit(1)
	}
	if isPfxSet {
		if len(pfx) == 0 {
			fmt.Fprintf(os.Stderr, "flag -%s must have non empty arg\n", addFlag)
			flag.Usage()
			os.Exit(1)
		}
	} else {
		pfx = pPub
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

func fName(suf string) string {
	return pfx + suf
}

func createIncs(first bool) (err error) {
	if first {
		if err = createNamedInc(fName(sConsts)); err != nil {
			return
		}
	} else {
		// this is possible only in full creation
		if err = createNamedInc(fName(sConsts), pPub+sInlines); err != nil {
			return
		}
	}
	if err = createNamedInc(fName(sTypes), fName(sConsts)); err != nil {
		return
	}
	if err = createNamedInc(fName(sInlines), fName(sTypes)); err != nil {
		return
	}
	return nil
}

func main() {
	checkFlags()
	if err := createIncs(true); err != nil {
		logErr(err)
	}
	if isPfxSet { // custom prefix
		if err := createNamedInc(pfx+".h", fName(sInlines)); err != nil { // export include
			logErr(err)
		}
		if err := createNamedSrc(pfx+".c", pfx+".h"); err != nil { // src template
			logErr(err)
		}
		return
	}
	if len(name) > 0 {
		if err := createNamedInc(name+".h", fName(sInlines)); err != nil { // export include
			logErr(err)
		}
	}
	if pub {
		return
	}
	pfx = pPriv
	if err := createIncs(false); err != nil {
		logErr(err)
	}
	if len(name) > 0 {
		if err := createNamedSrc(name+".c", fName(sInlines)); err != nil { // src template
			logErr(err)
		}
	}
}

func logErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	os.Exit(1)
}

func defName(iname string) string {
	n := includeName(iname)
	r := make([]byte, 0, len(n)+8)
	r = append(r, '_', '_')
	for _, b := range []byte(n) {
		switch {
		case b >= '0' && b <= '9':
			r = append(r, b)
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
