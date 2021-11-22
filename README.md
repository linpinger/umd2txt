# umd2txt

- 缘起: 收到网友`guo77`的邮件: `因为UMD文件自带目录，之前多年屯了很多UMD格式电子书，现在想转换，感觉重新设定目录十分麻烦，calibre曾经有人写过umd导入插件，但那个插件随着版本更新已经无效了`

- 功能: 读取umd格式，转为txt,fml,epub,mobi格式

- 参考: umd格式主要参考这里: (https://blog.csdn.net/lcchuan/article/details/6611898)

- txt: utf-8编码，unix换行符，前面会包含书名，作者等信息，章节名前有两个#号，方便使用正则表达式定位

- fml: 这个格式是自己模仿xml写的文本标记格式，可以用我的其他工具 `FoxBook` 来查看，编辑，转换为txt,mobi,pdf之类的

- 用法: `umd2txt xxx.umd` 或 `umd2txt -e epub xxx.umd`，使用`umd2txt -h`查看简单帮助

## 日志

- 2021-11-22: 从分段读取文件改为一次性读取到[]byte
- 2021-11-21: 将UMDReader分离
- 2021-11-20: 加入转换为epub/mobi(需存在kindlegen)功能
- 2021-11-20: 第一版 可转换为txt或fml格式

