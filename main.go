package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/linpinger/foxbook-golang/ebook"
)

const verDate string = "2021-11-22"

func main() {
	var lineHeadStr string // 转mobi/epub时每行开头添加的字符串
	var umdPath string
	var outFormat string
	var bLog bool
	flag.StringVar(&umdPath, "i", "", "umd File Path")
	flag.StringVar(&lineHeadStr, "s", "", "when create epub/mobi, each line head add this String, can be:　　")
	flag.StringVar(&outFormat, "e", "txt", "save format: txt, fml, epub, mobi")
	flag.BoolVar(&bLog, "l", false, "show debug log. version: "+verDate+" author: 觉渐(爱尔兰之狐)")
	flag.Parse() // 处理参数

	if 1 == flag.NArg() { // 处理后的参数个数，一般是文件路径
		umdPath = flag.Arg(0)
	} else {
		fmt.Println("# usage: umd2ebook -h")
		os.Exit(0)
	}
	if !bLog {
		log.SetOutput(ioutil.Discard)
	}

	log.Println("# start")

	umd := NewUMDReader(umdPath)

	// 导出标题，正文
	var eBook *ebook.EBook
	var buf bytes.Buffer
	switch outFormat {
	case "txt":
		buf.WriteString(fmt.Sprintln("书名:", umd.GetBookName()))
		buf.WriteString(fmt.Sprintln("作者:", umd.GetAuthorName()))
		buf.WriteString(fmt.Sprintln("日期:", umd.GetInfoDate()))
		buf.WriteString(fmt.Sprintln("类型:", umd.GetInfoType()))
		buf.WriteString(fmt.Sprintln("出版:", umd.GetInfoPub()))
		buf.WriteString(fmt.Sprintln("零售:", umd.GetInfoDist()))
		buf.WriteString("\n\n")
	case "fml":
		buf.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n\n<shelf>\n\n")
		buf.WriteString("<novel>\n\t<bookname>")
		buf.WriteString(umd.GetBookName())
		buf.WriteString("</bookname>\n\t<bookurl>file:///")
		buf.WriteString(strings.Replace(umd.GetUMDPath(), "\\", "/", -1))
		buf.WriteString(fmt.Sprintf("?date=%s&type=%s&pub=%s&dist=%s", umd.GetInfoDate(), umd.GetInfoType(), umd.GetInfoPub(), umd.GetInfoDist()))
		buf.WriteString("</bookurl>\n\t<delurl></delurl>\n\t<statu>0</statu>\n\t<qidianBookID></qidianBookID>\n\t<author>")
		buf.WriteString(umd.GetAuthorName())
		buf.WriteString("</author>\n<chapters>\n")
	case "epub":
		eBook = ebook.NewEBook(umd.GetBookName(), filepath.Join(umd.GetUMDDir(), "FoxEBookTmpDir"))
		eBook.SetAuthor(umd.GetAuthorName())
		if "" != umd.GetCoverPath() {
			eBook.SetCover(umd.GetCoverPath())
		}
	case "mobi":
		eBook = ebook.NewEBook(umd.GetBookName(), filepath.Join(umd.GetUMDDir(), "FoxEBookTmpDir"))
		eBook.SetAuthor(umd.GetAuthorName())
		if "" != umd.GetCoverPath() {
			eBook.SetCover(umd.GetCoverPath())
		}
	}
	pageCount := umd.GetChapterCount()
	for i := 0; i < pageCount; i++ {
		title, page := umd.GetTitleAndContentAt(i)

		switch outFormat {
		case "txt":
			buf.WriteString("\n## ")
			buf.WriteString(title)
			buf.WriteString("\n\n")
			buf.WriteString(page)
			buf.WriteString("\n\n")
		case "fml":
			buf.WriteString("<page>\n\t<pagename>")
			buf.WriteString(title)
			buf.WriteString("</pagename>\n\t<pageurl></pageurl>\n\t<content>")
			buf.WriteString(page)
			buf.WriteString("</content>\n\t<size>")
			buf.WriteString(strconv.Itoa(len(page)))
			buf.WriteString("</size>\n</page>\n")
		case "epub":
			if strings.Contains(page, "<br />") || strings.Contains(page, "<p>") || strings.Contains(page, "<br/>") {
				eBook.AddChapter(title, page, 1)
			} else {
				page = strings.Replace(page, " ", "&nbsp;", -1)
				nc := ""
				for _, line := range strings.Split(page, "\n") {
					nc = nc + lineHeadStr + line + "<br />\n"
				}
				eBook.AddChapter(title, nc, 1)
			}
		case "mobi":
			if strings.Contains(page, "<br />") || strings.Contains(page, "<p>") || strings.Contains(page, "<br/>") {
				eBook.AddChapter(title, page, 1)
			} else {
				page = strings.Replace(page, " ", "&nbsp;", -1)
				nc := ""
				for _, line := range strings.Split(page, "\n") {
					nc = nc + lineHeadStr + line + "<br />\n"
				}
				eBook.AddChapter(title, nc, 1)
			}
		}
		log.Println("- 标题:", title)
		log.Println("- 内容:", page)
	}
	switch outFormat {
	case "txt":
		os.WriteFile(filepath.Join(umd.GetUMDDir(), umd.GetUMDNameNoExt()+".txt"), buf.Bytes(), 0666)
	case "fml":
		buf.WriteString("</chapters>\n</novel>\n\n")
		buf.WriteString("</shelf>\n")
		os.WriteFile(filepath.Join(umd.GetUMDDir(), umd.GetUMDNameNoExt()+".fml"), buf.Bytes(), 0666)
	case "epub":
		eBook.SaveTo(filepath.Join(umd.GetUMDDir(), umd.GetUMDNameNoExt()+".epub"))
		if "" != umd.GetCoverPath() {
			os.Remove(umd.GetCoverPath())
		}
	case "mobi":
		eBook.SaveTo(filepath.Join(umd.GetUMDDir(), umd.GetUMDNameNoExt()+".mobi"))
		if "" != umd.GetCoverPath() {
			os.Remove(umd.GetCoverPath())
		}
	}

}
