package main

type changeLogStruct struct {
	Version   string `json:"version"`
	ChangeLog string `json:"changeLog"`
}

var changeLog = []changeLogStruct{
	{
		Version: "2024-10-09",
		ChangeLog: `feat(niuniu):在冷却还剩15s以下时不再提醒用户 feat(niuniu):多条打脚/jj/注册注销牛牛消息会合起来发(60s一次) change(niuniu):打胶5分钟CD，jj10分钟CD RikaCelery A minute ago
优化代码结构 RikaCelery 56 minutes ago
playwright auto wait all image loaded RikaCelery Today 00:41
rm docker actions RikaCelery Yesterday 16:08
change(dish): 怎么做和烹饪改为命令式(加{prefix}) change(driftbottle): 漂流瓶相关改为命令式(加{prefix}) RikaCelery Yesterday 15:57
fix lint error RikaCelery Yesterday 15:41
fix wrong merge RikaCelery Yesterday 15:12
更新依赖 RikaCelery Yesterday 15:10
Merge remote-tracking branch 'upstream/master' RikaCelery Yesterday 15:06
更新依赖 RikaCelery Yesterday 15:00
fix(github): 修复正则，移除无用代码 RikaCelery Yesterday 14:59
fix(niuniu): 修复类型转换错误 RikaCelery Yesterday 14:59
fix(github): 修正github链接识别正则 RikaCelery Yesterday 12:36
更新依赖 RikaCelery Yesterday 12:24
feat(github): 自动监听并渲染发送的GitHub链接 RikaCelery Yesterday 12:24
公开playwrightOptions RikaCelery Yesterday 12:24
fix(emojimix): 仅在混合时发送不支持提醒 RikaCelery Yesterday 12:23
fix(niuniu): api update RikaCelery Yesterday 12:22
fix(dish): 修复客官名显示为菜名的问题 (#1000) 昔音幻离* Yesterday 01:45
fix: 牛牛逻辑问题 (#996) 宇~* 2024/10/9 20:59
fix(emojimix): 修复在不支持的混合时报错 RikaCelery 2024/10/9 16:34
feat: 集成playwright-go环境 feat(browser): 截图插件(默认禁用) RikaCelery 2024/10/9 16:08
fix(niuniu): 牛牛插件无法jj fix(niuniu): 牛牛插件无法识别jj道具 RikaCelery 2024/10/9 09:31
replace action email RikaCelery 2024/10/9 01:09
remove webctrl RikaCelery 2024/10/9 01:03
update depend RikaCelery 2024/10/9 00:41`,
	},
	{
		Version: "2024-10-08",
		ChangeLog: `
feat(emojimix): 增加 {prefix}表情 获取表情动图
chore(lint): lint no error && enable lint RikaCelery 2024/10/8 23:12
fix(emojimix): 更改错误消息 RikaCelery 2024/10/8 22:39
Create dependabot.yml RikaCelery* 2024/10/9 00:41
disable lint and docker image RikaCelery 2024/10/8 16:25
v2024-10-08 RikaCelery 2024/10/8 16:21
fix(emojimix): 修复手机没动画 webp => gif RikaCelery 2024/10/8 10:08
fix(emojimix): fix NPE RikaCelery 2024/10/8 09:43
fix(emojimix): 表情混合只识别仅包含2个表情的消息 RikaCelery 2024/10/8 09:30
chore: update dependency RikaCelery 2024/10/8 09:24
fix(emojimix): 表情混合只识别仅包含2个表情的消息 feat(emojimix): 表情混合增加发送动图表情功能 RikaCelery 2024/10/8 09:01
fix(emojimix): 表情混合只识别仅包含2个表情的消息 RikaCelery 2024/10/8 08:48
`,
	},
	{
		Version: "2024-10-07",
		ChangeLog: `合并了ZeroBotPlugin上游，更新很多:
fix: regex error (#965)
feat: 新插件 牛牛大作战 (#944)
fix&feat(niuniu): 添加新玩法赎牛牛 (#970)
fix:修改niuniu插件at功能正则，提高兼容性 (#973)
fix(score): 签到图片余额为0(#978) (#979)
fix&feat(niuniu): 修复已知问题，添加新玩法牛牛商店 (#974)
fix: 修正niuniu的部分逻辑 (#981)
fix: 牛牛为负数时jj时的错误 (#984)
feat(manager): add slow send (#985)
fix(manager): remove fake sender
fix(manager): forward send
fix: aireply: 修复文字回复模式 (#991)
optimize(mcfish): 限制鱼贩的垄断 (#992)
reactor(emojimix): 更改提取emoji的算法，重构代码，提取函数
feat(emojimix): 增加{prefix}命令合成
feat(emojimix): 增加调用限制
`,
	},
	{
		Version: "2024-09-16",
		ChangeLog: `
feat(huntercode): support sender information

change(huntercode): {prefix}世界|崛起 在不写集会码时候默认显示所有集会码

fix(qqwife): 允许设置小数CD
fix(main): 消息过滤器不应该Block匹配群组
fix(qqwife): 当小三无法正确响应
fix(qqwife): 当小三指令无反应

refactor(qqwife): 使用新的消息解析方式

chore(main): 优化log格式
`,
	},
	{
		Version: "2024-09-06",
		ChangeLog: `
fix(huntercode): 自动删除上一天的集会码
fix(spider): 忽略无法解析的apk图标，继续解析其他字段
fix(main): 尝试从崩溃中回复错误信息

feat(spider): 支持更多信息解析
`,
	},
	{
		Version: "2024-09-04",
		ChangeLog: `新功能：
系统插件/spider: 现在可以自动检测APK文件并获取名字等相关信息
`,
	},
	{Version: "2024-08-26_1",
		ChangeLog: `新功能：
插件/guessmusic: 多线程下载
+ 指令: /report: 回复一条消息，快速反馈错误

修复：
插件/huntercode: 索引越界
插件/huntercode: 默认非公开
插件/guessmusic: 猜歌支持新网易云分享链接
`},
	{
		Version: "2024-08-26",
		ChangeLog: `新功能：
+ 插件/huntercode: 怪猎集会码分享管理
+ 插件/igem: 一次性截图所有wiki页面

变化：
+ 插件/quote: 消息记录中含有回复消息时自动查找被回复消息（这可能会导致实际渲染的消息多余指定的消息数量）
+ 插件/saucenao: 由关键词匹配改为前缀匹配，不会再对分享的小程序做错误响应

修复:
+ 插件/qqwife: 做媒功能无法识别@信息
+ 插件/资源嗅探: 无法正确识别被回复消息id`,
	},
	{
		Version: "2024-08-23_1",
		ChangeLog: `新功能：
+ 插件/资源嗅探: 对转发消息附加摘要信息
+ 插件/资源嗅探: 自动识别转发消息的摘要并发送

变化：
+ 插件/资源嗅探: 不再显示为0的项目

修复:
+ 插件/图片收藏(picpick): 无法正确获取图片链接`,
	},
	{
		Version: "2024-08-23",
		ChangeLog: `新功能：
+ 在使用未知指令时尝试寻找对应插件并进行提示
+ {prefix}更新日志 [指定版本号,可省略]  显示更新日志
+ 插件/picpick: 图片收藏插件 

修复:
+ 插件/quote: 修复渲染历史消息时少一条`,
	},
}
