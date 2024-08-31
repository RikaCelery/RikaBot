package main

type changeLogStruct struct {
	Version   string `json:"version"`
	ChangeLog string `json:"changeLog"`
}

var changeLog = []changeLogStruct{
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
