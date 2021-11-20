package main

import (
	"bytes"
	"encoding/binary"
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

	"golang.org/x/text/encoding/unicode"
)

const verDate string = "2021-11-20"

/*
参考:
https://blog.csdn.net/lcchuan/article/details/6611898

*/
func main() {
	var umdPath string
	var outFormat string
	var bLog bool
	flag.StringVar(&umdPath, "i", "", "umd File Path")
	flag.StringVar(&outFormat, "e", "txt", "save format: txt, fml")
	flag.BoolVar(&bLog, "l", false, "show debug log. version: "+verDate+" author: 觉渐(爱尔兰之狐)")
	flag.Parse() // 处理参数

	if 1 == flag.NArg() { // 处理后的参数个数，一般是文件路径
		umdPath = flag.Arg(0)
	}
	if !filepath.IsAbs(umdPath) {
		umdAbsPath, err := filepath.Abs(umdPath)
		if err != nil {
			log.Println("# Error: get umd absolute Path:", err)
			os.Exit(1)
		}
		umdPath = umdAbsPath
	}
	outDir, umdName := filepath.Split(umdPath)
	umdNameNoExt := strings.Replace(umdName, filepath.Ext(umdPath), "", -1)

	if "" == umdName {
		fmt.Println("# usage: umd2ebook -h")
		os.Exit(0)
	}
	if !bLog {
		log.SetOutput(ioutil.Discard)
	}

	log.Println("# start")

	fileUMD, err := os.OpenFile(umdPath, os.O_RDONLY, 0644)
	if err != nil {
		log.Println("# Error: open File:", err)
		return
	}

	var dataOffList []uint32 = make([]uint32, 0) // 数据块偏移顺序列表，合并正文按这个先后顺序
	var contentBytes []byte = make([]byte, 0)    // 正文内容块
	var contentLen uint32                        // 正文长度
	var idContentBlocks uint32                   // 数据标识: 正文块列表
	var idTitleList uint32                       // 数据标识: 章节标题
	var titleList []string = make([]string, 0)   // 章节标题列表
	var idChapterList uint32                     // 数据标识: 章节偏移
	var offList []uint32 = make([]uint32, 0)     // 章节偏移列表
	var bHaveCover bool = false                  // 是否有封面
	var idCover uint32                           // 封面

	var (
		bookName   = ""
		authorName = ""
		strYear    = ""
		strMonth   = ""
		strDay     = ""
		strType    = ""
		strPub     = ""
		strDis     = ""
	)
	// 存储{数据标识:正文内容}
	var mapIDContent map[uint32][]byte
	mapIDContent = make(map[uint32][]byte)

	// umd格式MagicNum: 0x89 9B 9A DE
	var offset int64 = 4

	for {
		block := make([]byte, 9)
		_, err = fileUMD.ReadAt(block, offset)
		if err != nil {
			log.Println("# Error: read block:", offset, err)
			return
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
				return
			}

			switch funcID {
			case 1: //umd文件头
				if 1 == content[0] { // UMD文件类型（1-文本，2-漫画）
					log.Println("  - 文本umd文件头:", content)
				} else {
					log.Println("  # 非文本umd，退出:", content)
					break
				}
			case 2: //文件标题
				bookName = unicodeLBytes2String(content)
				log.Println("  - 文件标题:", bookName)
			case 3: // 作者
				authorName = unicodeLBytes2String(content)
				log.Println("  - 作者:", authorName)
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
				strType = unicodeLBytes2String(content)
				log.Println("  - 小说类型:", strType)
			case 8:
				strPub = unicodeLBytes2String(content)
				log.Println("  - 出版商:", strPub)
			case 9:
				strDis = unicodeLBytes2String(content)
				log.Println("  - 零售商:", strDis)
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
					titleList = append(titleList, unicodeLBytes2String(dataTitles[ttOffset+1:ttOffset+1+int(lenTitle)]))
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

				if bHaveCover {
					os.WriteFile("cover.jpg", mapIDContent[idCover], 0666)
				}

				// 按章节顺序导出标题，正文
				var buf bytes.Buffer
				switch outFormat {
				case "txt":
					buf.WriteString(fmt.Sprintln("书名:", bookName))
					buf.WriteString(fmt.Sprintln("作者:", authorName))
					buf.WriteString(fmt.Sprintln("日期:", strYear+"-"+strMonth+"-"+strDay))
					buf.WriteString(fmt.Sprintln("类型:", strType))
					buf.WriteString(fmt.Sprintln("出版:", strPub))
					buf.WriteString(fmt.Sprintln("零售:", strDis))
					buf.WriteString("\n\n")
				case "fml":
					buf.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n\n<shelf>\n\n")
					buf.WriteString("<novel>\n\t<bookname>")
					buf.WriteString(bookName)
					buf.WriteString("</bookname>\n\t<bookurl>")
					buf.WriteString(umdPath)
					buf.WriteString("</bookurl>\n\t<delurl></delurl>\n\t<statu>0</statu>\n\t<qidianBookID></qidianBookID>\n\t<author>")
					buf.WriteString(authorName)
					buf.WriteString("</author>\n<chapters>\n")
				}
				pageCount := len(titleList)
				for i, title := range titleList {
					var page string
					if pageCount == i+1 { // 最后一章
						page = unicodeLBytes2String(contentBytes[offList[i]:contentLen])
					} else {
						page = unicodeLBytes2String(contentBytes[offList[i]:offList[i+1]])
					}
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
					}
					log.Println("- 标题:", title)
					log.Println("- 内容:", page)
				}
				switch outFormat {
				case "txt":
					os.WriteFile(filepath.Join(outDir, umdNameNoExt+".txt"), buf.Bytes(), 0666)
				case "fml":
					buf.WriteString("</chapters>\n</novel>\n\n")
					buf.WriteString("</shelf>\n")
					os.WriteFile(filepath.Join(outDir, umdNameNoExt+".fml"), buf.Bytes(), 0666)
				}
				return
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
				return
			}
			mapIDContent[dataID] = content // 数据块存入map
			// fmt.Println("  - 数据内容:", content)

			offset += int64(dataLen)
		} else {
			log.Println("- 未知块:", block)
			break
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
