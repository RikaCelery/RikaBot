package main

type changeLogStruct struct {
	Version   string `json:"version"`
	ChangeLog string `json:"changeLog"`
}

var changeLog = []changeLogStruct{
	{
		Version: "2024-08-23",
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
