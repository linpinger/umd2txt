package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"compress/zlib"
	"io"
	"os"

	"github.com/linpinger/foxbook-golang/ebook"
	"golang.org/x/text/encoding/unicode"
)

const verDate string = "2021-11-21"

// 参考: https://blog.csdn.net/lcchuan/article/details/6611898

// --------- umd 解析库 by Fox
type UMDReader struct {
	BookName     string
	AuthorName   string
	InfoDate     string
	InfoType     string
	InfoPub      string
	InfoDist     string
	UMDPath      string
	UMDDir       string
	UMDNameNoExt string
	CoverPath    string
	TitleList    []string // 章节标题列表
	ContentList  []string // 章节内容列表
}

func NewUMDReader(umdPath string) *UMDReader {
	var umd UMDReader

	umd.UMDPath = umdPath
	umd.BookName = "书名"
	umd.AuthorName = "作者"
	umd.InfoDate = "2000-00-00"
	umd.InfoType = "类型"
	umd.InfoPub = "出版商"
	umd.InfoDist = "零售商"

	umd.CoverPath = ""

	umd.splitFilePath()
	umd.readUMD()

	return &umd
}
func (umd *UMDReader) GetBookName() string {
	return umd.BookName
}
func (umd *UMDReader) GetAuthorName() string {
	return umd.AuthorName
}
func (umd *UMDReader) GetInfoDate() string {
	return umd.InfoDate
}
func (umd *UMDReader) GetInfoPub() string {
	return umd.InfoPub
}
func (umd *UMDReader) GetInfoDist() string {
	return umd.InfoDist
}
func (umd *UMDReader) GetInfoType() string {
	return umd.InfoType
}

func (umd *UMDReader) splitFilePath() error {
	if !filepath.IsAbs(umd.UMDPath) {
		umdAbsPath, err := filepath.Abs(umd.UMDPath)
		if err != nil {
			log.Println("# Error: get umd absolute Path:", err)
			return err
		}
		umd.UMDPath = umdAbsPath
	}
	outDir, umdName := filepath.Split(umd.UMDPath)
	umdNameNoExt := strings.Replace(umdName, filepath.Ext(umd.UMDPath), "", -1)
	umd.UMDDir = outDir
	umd.UMDNameNoExt = umdNameNoExt
	return nil
}

// TODO: 一次性读入
func (umd *UMDReader) readUMD() error {
	fileUMD, err := os.OpenFile(umd.UMDPath, os.O_RDONLY, 0644)
	if err != nil {
		log.Println("# Error: open File:", err)
		return err
	}

	var dataOffList []uint32    // 数据块偏移顺序列表，合并正文按这个先后顺序
	var contentBytes []byte     // 正文内容块
	var contentLen uint32       // 正文长度
	var idContentBlocks uint32  // 数据标识: 正文块列表
	var idTitleList uint32      // 数据标识: 章节标题
	var idChapterList uint32    // 数据标识: 章节偏移
	var offList []uint32        // 章节偏移列表
	var bHaveCover bool = false // 是否有封面
	var idCover uint32          // 封面

	var (
		strYear  = ""
		strMonth = ""
		strDay   = ""
	)
	// 存储{数据标识:正文内容}
	var mapIDContent map[uint32][]byte = make(map[uint32][]byte)

	// umd格式MagicNum: 0x89 9B 9A DE
	var offset int64 = 4

	for {
		block := make([]byte, 9)
		_, err = fileUMD.ReadAt(block, offset)
		if err != nil {
			log.Println("# Error: read block:", offset, err)
			return err
		}

		if 35 == block[0] {
			log.Println("- 功能块:", block)
			funcID := block[1]
			log.Println("  - 功能ID:", funcID)
			funcLen := int64(block[4])
			log.Println("  - 功能Len:", funcLen)

			content := make([]byte, funcLen-5)
			_, err = fileUMD.ReadAt(content, offset+5)
			if err != nil {
				log.Println("# Error: read content:", offset+5, err)
				return err
			}

			switch funcID {
			case 1: //umd文件头
				if 1 == content[0] { // UMD文件类型（1-文本，2-漫画）
					log.Println("  - 文本umd文件头:", content)
				} else {
					log.Println("  # 非文本umd，退出:", content)
					return errors.New("# Error: not text type UMD")
				}
			case 2: //文件标题
				umd.BookName = unicodeLBytes2String(content)
				log.Println("  - 文件标题:", umd.BookName)
			case 3: // 作者
				umd.AuthorName = unicodeLBytes2String(content)
				log.Println("  - 作者:", umd.AuthorName)
			case 4:
				strYear = unicodeLBytes2String(content)
				log.Println("  - 年:", strYear)
			case 5:
				strMonth = unicodeLBytes2String(content)
				log.Println("  - 月:", strMonth)
			case 6:
				strDay = unicodeLBytes2String(content)
				log.Println("  - 日:", strDay)
			case 7:
				umd.InfoType = unicodeLBytes2String(content)
				log.Println("  - 小说类型:", umd.InfoType)
			case 8:
				umd.InfoPub = unicodeLBytes2String(content)
				log.Println("  - 出版商:", umd.InfoPub)
			case 9:
				umd.InfoDist = unicodeLBytes2String(content)
				log.Println("  - 零售商:", umd.InfoDist)
			case 10: // 0x0A CONTENT ID
				log.Println("  - 0x0A CONTENT ID:", content)
			case 11: // 0x0B 内容长度:小说未压缩时的内容总长度（字节）
				contentLen = bytes2Uint32(content)
				log.Println("  - 0x0B 内容长度:", contentLen, " = ", content)
			case 12: // 0x0C 文件结束:整个文件长度
				log.Println("  - 0x0C 文件结束:", content, " 文件长度 = ", bytes2Uint32(content))

				// 章节偏移列表: offList
				posListCount := len(mapIDContent[idChapterList]) / 4 // 章节偏移数
				for i := 0; i < posListCount; i++ {
					offList = append(offList, bytes2Uint32(mapIDContent[idChapterList][i*4:i*4+4]))
				}
				log.Println("- 章节数:", len(offList), "章节:", offList)

				// 章节标题列表: titleList
				dataTitles := mapIDContent[idTitleList] //章节标题
				lenData := len(dataTitles)
				ttOffset := 0
				for {
					lenTitle := uint8(dataTitles[ttOffset])
					umd.TitleList = append(umd.TitleList, unicodeLBytes2String(dataTitles[ttOffset+1:ttOffset+1+int(lenTitle)]))
					ttOffset += 1 + int(lenTitle)
					// fmt.Println("  - 章节标题:", unicodeLBytes2String(strTitle), " len:", lenTitle, "ttOffset:", ttOffset, ">=", lenData)
					if ttOffset >= lenData {
						break
					}
				}

				// 正文块列表
				dataContBlocks := mapIDContent[idContentBlocks] //正文块列表
				contBlockCount := len(dataContBlocks) / 4       // 章节偏移数
				var buffer bytes.Buffer

				var dataBlockList []uint32 = make([]uint32, 0)
				for i := 0; i < contBlockCount; i++ {
					id := bytes2Uint32(dataContBlocks[i*4 : i*4+4])
					dataBlockList = append(dataBlockList, id)
				}
				for _, blkOffset := range dataOffList {
					for _, id := range dataBlockList {
						if blkOffset == id {
							buffer.Write(uncompressBytes(mapIDContent[id]))
							log.Println("- zhenid:", id)
							// fmt.Println("- content id:", id, "Content:", unicodeLBytes2String(uncompressBytes(mapIDContent[id])))
						}
					}
				}
				// 0x2920 -> 0x0A00
				contentBytes = bytes.ReplaceAll(buffer.Bytes(), []byte{0x29, 0x20}, []byte{0x0A, 0x00})
				log.Println("- 正文块 Len:", len(contentBytes), "应该长度:", contentLen)
				// os.WriteFile("T:/content.bin", contentBytes[:contentLen], 0666)

				umd.InfoDate = strYear + "-" + strMonth + "-" + strDay
				if bHaveCover {
					umd.CoverPath = filepath.Join(umd.UMDDir, "cover.jpg")
					os.WriteFile(umd.CoverPath, mapIDContent[idCover], 0666)
				}

				// 按章节顺序导出标题，正文
				pageCount := len(umd.TitleList)
				for i, _ := range umd.TitleList {
					var page string
					if pageCount == i+1 { // 最后一章
						page = unicodeLBytes2String(contentBytes[offList[i]:contentLen])
					} else {
						page = unicodeLBytes2String(contentBytes[offList[i]:offList[i+1]])
					}
					umd.ContentList = append(umd.ContentList, page)
				}

				return nil
			case 129: // 0x81 正文结束: 指向正文索引数据块的RandVal
				idContentBlocks = bytes2Uint32(content)
				log.Println("  - 0x81 正文结束:", content)
			case 130: // 0x82 封面（jpg）
				if 1 == content[0] { // 1:jpg
					bHaveCover = true
					idCover = bytes2Uint32(content[1:])
				}
				log.Println("  - 0x82 封面（jpg）:", content)
			case 131: // 0x83 章节偏移: 里面存储的是各个章节在正文（解压后的文本）中的偏移，即表示第几章是从何处开始的。每个偏移使用4个字节，由此我们可以知道Content总一共存了（（Length-9）/4）个偏移。
				idChapterList = bytes2Uint32(content)
				log.Println("  - 0x83 章节偏移(数据标识):", content)
			case 132: // 0x84 章节标题，正文: 它的Content是一个简单模式的重复。这个模式就是：标题长度-标题内容。其中标题长度用1字节保存，标题内容紧跟在标题长度之后，内容也是UNICODE的，不过是明文，没压缩的
				idTitleList = bytes2Uint32(content)
				log.Println("  - 0x84 章节标题，正文:", content)
			case 135: // 0x87 页面偏移（Page Offset）:(1:字体大小, 2:屏幕宽度, 3-6:指向一个页面偏移数据块)
				log.Println("  - 0x87 页面偏移（Page Offset）:", content)
			case 240: // 0xF0 CDS KEY
				log.Println("  - 0xF0 CDS KEY:", content)
			case 241: // 0xF1 许可证(LICENCE KEY): 16字节空数据
				log.Println("  - 0xF1 许可证(LICENCE KEY):", content)
			default:
				log.Println("  - 功能ID:", funcID)
				log.Println("  - 功能内容:", content)
			}

			offset += funcLen
		} else if 36 == block[0] { // 数据块

			log.Println("- 数据块:", block)
			dataID := bytes2Uint32(block[1:5]) // 将所有数据标识都转为uint32，方便存入map ? 和查找
			dataOffList = append(dataOffList, dataID)
			log.Println("  - 数据标识:", block[1:5], " = uint32:", dataID)
			dataLen := bytes2Uint32(block[5:9])
			log.Println("  - 数据长度:", block[5:9], " = uint32:", dataLen)

			content := make([]byte, dataLen-9)
			_, err = fileUMD.ReadAt(content, offset+9)
			if err != nil {
				log.Println("# Error: read content:", offset+9, err)
				return err
			}
			mapIDContent[dataID] = content // 数据块存入map
			// fmt.Println("  - 数据内容:", content)

			offset += int64(dataLen)
		} else {
			log.Println("- 未知块:", block)
			return errors.New("# Error: uknown block: " + string(block))
		}
	}
	return nil
}

func (umd *UMDReader) GetChapterCount() int {
	return len(umd.TitleList)
}
func (umd *UMDReader) GetTitleAt(idx int) string {
	if idx < 0 || idx >= len(umd.TitleList) {
		return ""
	}
	return umd.TitleList[idx]
}
func (umd *UMDReader) GetContentAt(idx int) string {
	if idx < 0 || idx >= len(umd.ContentList) {
		return ""
	}
	return umd.ContentList[idx]
}

func (umd *UMDReader) GetCoverPath() string {
	return umd.CoverPath
}

// --------- umd 解析库 by Fox

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
		buf.WriteString(strings.Replace(umd.UMDPath, "\\", "/", -1))
		buf.WriteString(fmt.Sprintf("?date=%s&type=%s&pub=%s&dist=%s", umd.GetInfoDate(), umd.GetInfoType(), umd.GetInfoPub(), umd.GetInfoDist()))
		buf.WriteString("</bookurl>\n\t<delurl></delurl>\n\t<statu>0</statu>\n\t<qidianBookID></qidianBookID>\n\t<author>")
		buf.WriteString(umd.GetAuthorName())
		buf.WriteString("</author>\n<chapters>\n")
	case "epub":
		eBook = ebook.NewEBook(umd.GetBookName(), filepath.Join(umd.UMDDir, "FoxEBookTmpDir"))
		eBook.SetAuthor(umd.GetAuthorName())
		if "" != umd.GetCoverPath() {
			eBook.SetCover(umd.GetCoverPath())
		}
	case "mobi":
		eBook = ebook.NewEBook(umd.GetBookName(), filepath.Join(umd.UMDDir, "FoxEBookTmpDir"))
		eBook.SetAuthor(umd.GetAuthorName())
		if "" != umd.GetCoverPath() {
			eBook.SetCover(umd.GetCoverPath())
		}
	}
	pageCount := umd.GetChapterCount()
	for i := 0; i < pageCount; i++ {
		title := umd.GetTitleAt(i)
		page := umd.GetContentAt(i)

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
		os.WriteFile(filepath.Join(umd.UMDDir, umd.UMDNameNoExt+".txt"), buf.Bytes(), 0666)
	case "fml":
		buf.WriteString("</chapters>\n</novel>\n\n")
		buf.WriteString("</shelf>\n")
		os.WriteFile(filepath.Join(umd.UMDDir, umd.UMDNameNoExt+".fml"), buf.Bytes(), 0666)
	case "epub":
		eBook.SaveTo(filepath.Join(umd.UMDDir, umd.UMDNameNoExt+".epub"))
		if "" != umd.GetCoverPath() {
			os.Remove(umd.GetCoverPath())
		}
	case "mobi":
		eBook.SaveTo(filepath.Join(umd.UMDDir, umd.UMDNameNoExt+".mobi"))
		if "" != umd.GetCoverPath() {
			os.Remove(umd.GetCoverPath())
		}
	}

}

func unicodeLBytes2String(iBytes []byte) string { // unicode LittleEndian 的 bytes 转string
	utf8Bytes, _ := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder().Bytes(iBytes)
	return string(utf8Bytes)
}

func bytes2Uint32(iBytes []byte) uint32 { // 4字节的[]byte 转 uint32
	var cLen uint32
	buf := bytes.NewReader(iBytes)
	binary.Read(buf, binary.LittleEndian, &cLen)
	return cLen
}

func uncompressBytes(iBytes []byte) []byte {
	var outBuf bytes.Buffer
	b := bytes.NewReader(iBytes)
	r, err := zlib.NewReader(b)
	if err != nil {
		fmt.Println("# Error: uncompress:", err)
	}
	io.Copy(&outBuf, r)
	r.Close()
	return outBuf.Bytes()
}
