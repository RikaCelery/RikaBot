package main

type changeLogStruct struct {
	Version   string `json:"version"`
	ChangeLog string `json:"changeLog"`
}

var changeLog = []changeLogStruct{
	{
		Version: "2024-11-15",
		ChangeLog: `feat(previewer): add quality configuration
feat: nick configurable
fix(playwright): 修复等待和调整页面高度顺序 
feat(previewer): 支持从配置文件加载 
fix(playwright): 修复全屏截图不能正确计算某些页面的高度 
fix(rss): template read error 
feat(previewer): twitter preview 
feat(previewer): gen previews 
change(chatcount): no 水群提醒 
feat(browser): shell arg parse 
fix(playwright): fix template function bug 
change(browser): require admin permission 
fix(slash): wrong regex 
添加撤回插件 
fix(browser): panic error fix(browser): closed context error 
fix(github): do not response topics 
fix(petpet): no panic on init failed 
fix(quote): pattern matcher change(quote): remove auto lookup replied message 
feat(playwright): no cache for qlogo.cn 
feat(petpet): 新的摸摸头插件（petpet）
feat(utils)file r/w shortcuts 
feat(qqwife): 添加好感度提升途径 (#1049) 
fix: 进行1e5次钓鱼不出下界合金竿的问题 (#1051) 
fix(niuniu): 一些小问题 (#1043) 
feat(niuniu): 寫真으로 順位 表示 (#1024) 
feat(spider): file downloader 
refactor(spider): 优化代码结构 
fix(slash): 不响应@xxx /xxx（避免机器人误触） 
feat(emojimix): 对于某些表情自动替换为相近的有动画的表情 
feat(slash): 响应诸如 “/打”、“/敲” “/prpr” 之类的消息 
fix(huntercode): /世界 /崛起 没反应 
fix(emojimix):表情混合修改消息导致后续插件无法工作 
fix real-cugan 
fix:修复猜单词插件最后一轮无法正常发送的错误 (#1039) 
fix:修复出售限制未生效的问题 (#1038) 
help(emozi): 增加注册提示 
feat: add plugin emozi(抽象转写) & remove vitsnyaru 
feat(manager): no forward on single slow 
fix(playwright): 修复滑动错误 fix(playwright): 修复代理问题 feat(playwright): 增加预处理js/CSS用于移除无用元素 
fix: 重写交易鱼类上限逻辑 (#1002) (#1003) 
fix(spider): 不对5张一下图片或视频的转发聊天响应，避免扰民`,
	},
	{
		Version: "2024-10-09",
		ChangeLog: `feat(niuniu):在冷却还剩15s以下时不再提醒用户 feat(niuniu):多条打脚/jj/注册注销牛牛消息会合起来发(60s一次) change(niuniu):打胶5分钟CD，jj10分钟CD RikaCelery A minute ago
优化代码结构 RikaCelery 56 minutes ago
playwright auto wait all image loaded 
rm docker actions 
change(dish): 怎么做和烹饪改为命令式(加{prefix}) change(driftbottle): 漂流瓶相关改为命令式(加{prefix}) 
fix lint error 
fix wrong merge 
更新依赖 
更新依赖 
fix(github): 修复正则，移除无用代码 
fix(niuniu): 修复类型转换错误 
fix(github): 修正github链接识别正则 
更新依赖 
feat(github): 自动监听并渲染发送的GitHub链接 
公开playwrightOptions 
fix(emojimix): 仅在混合时发送不支持提醒 
fix(niuniu): api update 
fix(dish): 修复客官名显示为菜名的问题 (#1000) 
fix: 牛牛逻辑问题 (#996) 
fix(emojimix): 修复在不支持的混合时报错 
feat: 集成playwright-go环境 feat(browser): 截图插件(默认禁用) 
fix(niuniu): 牛牛插件无法jj fix(niuniu): 牛牛插件无法识别jj道具 
replace action email 
remove webctrl 
update depend `,
	},
	{
		Version: "2024-10-08",
		ChangeLog: `
feat(emojimix): 增加 {prefix}表情 获取表情动图
fix(emojimix): 更改错误消息 
Create dependabot.yml 
disable lint and docker image 
v2024-10-08 
fix(emojimix): 修复手机没动画 webp => gif 
fix(emojimix): fix NPE 
fix(emojimix): 表情混合只识别仅包含2个表情的消息 
fix(emojimix): 表情混合只识别仅包含2个表情的消息 feat(emojimix): 表情混合增加发送动图表情功能 
fix(emojimix): 表情混合只识别仅包含2个表情的消息 
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
