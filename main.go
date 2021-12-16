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

	"github.com/linpinger/golib/ebook"
)

const verDate string = "2021-12-13"

func main() {
	var lineHeadStr string // 转mobi/epub时每行开头添加的字符串
	var umdPath string
	var outFormat string
	var bLog bool
	flag.StringVar(&umdPath, "i", "", "umd File Path")
	flag.StringVar(&lineHeadStr, "s", "", "when create epub/mobi, each line head add this String, can be:　　")
	flag.StringVar(&outFormat, "e", "txt", "save format: txt, fml, epub, mobi, azw3")
	flag.BoolVar(&bLog, "l", false, "show debug log. version: "+verDate+" author: 觉渐(爱尔兰之狐)")
	flag.Parse() // 处理参数

	if 1 == flag.NArg() { // 处理后的参数个数，一般是文件路径
		umdPath = flag.Arg(0)
	}
	if "" == umdPath {
		fmt.Println("# usage: umd2ebook -h")
		os.Exit(0)
	}
	if !bLog {
		log.SetOutput(ioutil.Discard)
	}

	log.Println("# start")

	umd := ebook.NewUMDReader(umdPath)
	eBookSavePath := filepath.Join(umd.GetUMDDir(), umd.GetUMDNameNoExt()+"."+outFormat)

	// 导出标题，正文
	var bk *ebook.EPubWriter
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
	default:
		bk = ebook.NewEPubWriter(umd.GetBookName(), eBookSavePath)
		bk.SetTempDir(umd.GetUMDDir())
		bk.SetAuthor(umd.GetAuthorName())
		if "" != umd.GetCoverPath() {
			bk.SetCover(umd.GetCoverPath())
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
		default:
			if strings.Contains(page, "<br />") || strings.Contains(page, "<p>") || strings.Contains(page, "<br/>") {
				bk.AddChapter(title, page)
			} else {
				page = strings.Replace(page, " ", "&nbsp;", -1)
				nc := ""
				for _, line := range strings.Split(page, "\n") {
					nc = nc + lineHeadStr + line + "<br />\n"
				}
				bk.AddChapter(title, nc)
			}
		}
		log.Println("- 标题:", title)
		log.Println("- 内容:", page)
	}
	switch outFormat {
	case "txt":
		os.WriteFile(eBookSavePath, buf.Bytes(), 0666)
	case "fml":
		buf.WriteString("</chapters>\n</novel>\n\n")
		buf.WriteString("</shelf>\n")
		os.WriteFile(eBookSavePath, buf.Bytes(), 0666)
	default:
		bk.SetMobiUseHideArg()
		bk.SaveTo()
		if "" != umd.GetCoverPath() {
			os.Remove(umd.GetCoverPath())
		}
	}

}
