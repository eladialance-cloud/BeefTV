package manju

const (
	StyleCatalogVersion = "V7.4.26"
	StyleCustomCardID   = "custom"

	StyleFamilyT1 = "T1"
	StyleFamilyT2 = "T2"
	StyleFamilyT3 = "T3"
	StyleFamilyT4 = "T4"
	StyleFamilyT5 = "T5"
	StyleFamilyT6 = "T6"
	StyleFamilyT7 = "T7"
	StyleFamilyT8 = "T8"
)

type StyleBaseDefinition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type StyleFamilyDefinition struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Feature string `json:"feature"`
}

type StyleCard struct {
	ID                  string   `json:"id"`
	Number              int      `json:"number"`
	Name                string   `json:"name"`
	Group               string   `json:"group"`
	Family              string   `json:"family"`
	FightRating         int      `json:"fightRating"`
	EffectRating        int      `json:"effectRating"`
	Grade               string   `json:"grade"`
	NativeGenre         string   `json:"nativeGenre"`
	PromptParameters    string   `json:"promptParameters"`
	AvoidanceWarning    string   `json:"avoidanceWarning"`
	CompatibleStyleBase []string `json:"compatibleStyleBases"`
}

type StyleCatalogView struct {
	Version  string                  `json:"version"`
	Bases    []StyleBaseDefinition   `json:"bases"`
	Families []StyleFamilyDefinition `json:"families"`
	Items    []StyleCard             `json:"items"`
}

var styleBaseDefinitions = []StyleBaseDefinition{
	{ID: StyleBaseLiveAction, Name: "真人实拍质感"},
	{ID: StyleBase2DAnimation, Name: "2D 手绘动画"},
	{ID: StyleBase3DCG, Name: "3D CG 动画"},
	{ID: StyleBaseInk2D, Name: "水墨二维"},
}

var styleFamilyDefinitions = []StyleFamilyDefinition{
	{ID: StyleFamilyT1, Name: "刃光派", Feature: "高反差硬光 · 强轮廓 · 广角低机位仰拍 · 升格慢镜"},
	{ID: StyleFamilyT2, Name: "冷峻秩序派", Feature: "冷蓝灰调 · 中轴对称 · 几何分割 · 低照度"},
	{ID: StyleFamilyT3, Name: "柔光写实派", Feature: "窗侧自然光 · 低对比 · 中近景 · 克制表演"},
	{ID: StyleFamilyT4, Name: "浓艳装饰派", Feature: "大色块 · 高饱和 · 对称仪式感 · 民艺纹样"},
	{ID: StyleFamilyT5, Name: "纪实手持派", Feature: "粗粝手持 · 自然光 · 灰土调 · 颗粒不修饰"},
	{ID: StyleFamilyT6, Name: "极端写意派", Feature: "大留白 · 极简 · 非自然色 · 极长镜"},
	{ID: StyleFamilyT7, Name: "神话史诗派", Feature: "巨大体量对比 · 逆光尘雾 · 图腾兽纹 · 硬表面机械"},
	{ID: StyleFamilyT8, Name: "动画与国漫3D/CG派", Feature: "次世代国漫3D建模 · 游戏CG · 三渲二进阶水墨 · 2D厚涂赛璐璐"},
}

var styleFamilyBaseDefaults = map[string][]string{
	StyleFamilyT1: {StyleBaseLiveAction, StyleBase2DAnimation, StyleBase3DCG},
	StyleFamilyT2: {StyleBaseLiveAction, StyleBase3DCG},
	StyleFamilyT3: {StyleBaseLiveAction, StyleBase2DAnimation},
	StyleFamilyT4: {StyleBaseLiveAction, StyleBase2DAnimation, StyleBase3DCG},
	StyleFamilyT5: {StyleBaseLiveAction},
	StyleFamilyT6: {StyleBaseLiveAction, StyleBase2DAnimation, StyleBaseInk2D},
	StyleFamilyT7: {StyleBaseLiveAction, StyleBase3DCG},
	StyleFamilyT8: {StyleBase2DAnimation, StyleBase3DCG, StyleBaseInk2D},
}

// styleCards is the complete V7.4.26 option-layer catalog. Retired numbers
// 67 and 70 are deliberately absent. Names remain internal ledger data and
// must never enter model-facing prompts; PromptParameters is the reusable
// translation that downstream production consumes.
var styleCards = []StyleCard{
	styleCard(1, "张艺谋", "中国内地", StyleFamilyT4, 1, 2, "A", "仪式叙事", "极高饱和色块 · 对称构图 · 大红/靛青对比 · 民艺纹样 · 群像横移 · 浓云硬光", "配色过艳 / 人物失焦"),
	styleCard(2, "陈凯歌", "中国内地", StyleFamilyT7, 2, 2, "A", "厚重史诗", "巨景小人 · 逆光沙尘 · 金红与褐 · 极慢横移 · 沉重节奏", "史诗尺度压垮短剧 / 人物过小"),
	styleCard(3, "冯小刚", "中国内地", StyleFamilyT3, 1, 1, "A", "市民生活流", "中焦对白 · 家居实景 · 冷蓝夜街与暖黄室内 · 快速对话切换", "动作承载弱 / 场景过于生活化"),
	styleCard(4, "周星驰", "中国内地", StyleFamilyT4, 1, 1, "A", "港式喜剧", "广角畸变 · 夸张表情 · 高饱和 · 快节奏 · 前景遮挡笑点", "表情失真 / 只剩夸张没有叙事"),
	styleCard(5, "宁浩", "中国内地", StyleFamilyT5, 1, 1, "B", "类型拼贴", "多段质感拼接 · 快剪 · 类型符号强化 · 章节感", "风格拼接断裂 / 段落不统一"),
	styleCard(6, "乌尔善", "中国内地", StyleFamilyT7, 3, 3, "A", "上古神话史诗", "岩画兽纹 · 巨大体量对比 · 土红与骨白 · 篝火暖光 · 庄严缓推", "西方奇幻铠甲 / 廉价土黄"),
	styleCard(7, "路阳", "中国内地", StyleFamilyT1, 3, 2, "B", "武侠动作", "冷青灰配烛火暖橙 · 低机位贴身跟拍 · 快刃三五合", "长镜过难须拆跟拍切镜 / 禁五彩仙侠光球"),
	styleCard(8, "毕赣", "中国内地", StyleFamilyT6, 0, 0, "C", "文艺诗电影", "潮湿雾气 · 隧道霓虹 · 长焦压缩 · 时间失速", "时间失速，无动作语法"),
	styleCard(9, "徐克", "中国香港", StyleFamilyT1, 3, 3, "B", "武侠玄幻", "高反差大光比 · 强轮廓硬光 · 冷蓝主调与暖金点光 · 体积雾 · 广角低机位仰拍 · 120fps 升格", "塑料感三渲二 / 游戏CG均匀打光"),
	styleCard(10, "王家卫", "中国香港", StyleFamilyT2, 0, 0, "B", "都市情感", "抽帧残影 · 高饱和霓虹与暖黄交错 · 狭窄空间 · 贴身手持 · 前景遮挡 · 光晕渗化 · 慢门拖影", "参数组禁粒子禁高帧率，打戏禁用"),
	styleCard(11, "杜琪峰", "中国香港", StyleFamilyT2, 0, 1, "A", "都市写实", "长焦压缩 · 冷静机位 · 疏离构图 · 灰蓝调 · 大留白", "节奏过平 / 缺情绪抓手"),
	styleCard(12, "袁和平", "中国香港", StyleFamilyT1, 3, 2, "B", "武侠动作", "兵器形制特写 · 布料与皮甲质感 · 真实重心与发力 · 汗与尘", "招式堆砌 / 禁隔空对波"),
	styleCard(13, "彭顺", "中国香港", StyleFamilyT2, 1, 1, "B", "港式惊悚", "逼仄空间 · 冷绿光 · 手持晃动 · 低照度 · 局部高光", "过暗丢失动作 / 惊悚压过人物"),
	styleCard(14, "陈可辛", "中国香港", StyleFamilyT3, 0, 1, "A", "都市情感", "暖调柔光 · 生活实景 · 中近景 · 手持轻晃 · 情绪特写", "动作弱 / 容易变成生活剧"),
	styleCard(15, "李少红", "中国香港", StyleFamilyT4, 0, 1, "A", "古典唯美", "柔焦 · 浓艳 · 绸缎光泽 · 对称帘幕 · 烛光暖影", "过甜 / 缺质感"),
	styleCard(16, "刘镇伟", "中国香港", StyleFamilyT4, 1, 2, "A", "港式奇幻", "高饱和奇幻 · 柔光人像 · 蓝紫幻光 · 星芒", "廉价影楼感"),
	styleCard(17, "李安", "中国台湾", StyleFamilyT3, 0, 1, "A", "克制写实", "自然侧光 · 中景为主 · 低饱和 · 留白 · 克制表演", "节奏过缓 / 缺视觉冲击"),
	styleCard(18, "侯孝贤", "中国台湾", StyleFamilyT5, 0, 0, "B", "文艺写实", "固定远景长镜 · 真实环境光 · 无戏剧化打光 · 平实构图", "打斗会被看淡"),
	styleCard(19, "杨德昌", "中国台湾", StyleFamilyT2, 0, 0, "A", "都市疏离", "中焦 · 冷青调 · 前后景深隔断 · 快速跳切", "人物关系不清 / 节奏偏冷"),
	styleCard(20, "蔡明亮", "中国台湾", StyleFamilyT6, 0, 0, "B", "极简写实", "极低机位 · 手持 · 潮湿暗调 · 窗框遮挡 · 慢节奏", "与打斗节奏对立"),
	styleCard(21, "王童", "中国台湾", StyleFamilyT4, 1, 1, "A", "视觉华丽", "民间装饰纹样 · 浓饱和 · 对称 · 图画性构图", "画面拥挤 / 缺纵深"),
	styleCard(22, "黑泽明", "日本", StyleFamilyT7, 3, 1, "A", "时代剧", "自然硬光 · 实景搭景 · 多机位群像 · 明快色彩 · 剪影构图", "无特效传统 / 需另补"),
	styleCard(23, "北野武", "日本", StyleFamilyT6, 1, 0, "C", "极简暴力", "极简固定镜 · 大段空镜 · 海浪与风 · 骤停画格", "打斗会被剪成静止"),
	styleCard(24, "今村昌平", "日本", StyleFamilyT5, 1, 0, "B", "社会写实", "人群挤压 · 自然光 · 土色系 · 群体动作混乱感", "秩序失控 / 主体被淹没"),
	styleCard(25, "是枝裕和", "日本", StyleFamilyT3, 0, 0, "A", "家庭日常", "家庭实景 · 窗侧自然光 · 中近景 · 低对比 · 停顿", "无动作 / 情绪平"),
	styleCard(26, "河濑直美", "日本", StyleFamilyT3, 0, 0, "A", "抒情", "逆光柔焦 · 淡曝光 · 手持微晃 · 浅景深", "无动作 / 画面过淡"),
	styleCard(27, "岩井俊二", "日本", StyleFamilyT3, 0, 0, "A", "青春回忆", "逆光光晕 · 柔焦 · 胶片青绿 · 慢节奏", "无动作 / 回忆质感过度"),
	styleCard(28, "押井守", "日本", StyleFamilyT8, 2, 3, "B", "赛博科幻", "冷青 · 玻璃幕墙反射 · 长焦 · 静止构图 · 2D/3D 混合质感", "过于静止 / 混合质感不统一", StyleBase2DAnimation, StyleBase3DCG),
	styleCard(29, "大友克洋", "日本", StyleFamilyT7, 2, 3, "A", "机甲科幻", "冷金属机械 · 微距 · 硬表面反射 · 精密结构", "机械过密 / 人物被吞"),
	styleCard(30, "铃木敏郎", "日本", StyleFamilyT3, 0, 0, "A", "生活质感", "水彩手绘 · 柔和日光 · 淡雅彩 · 生活器物", "无动作 / 手绘质感不稳", StyleBase2DAnimation),
	styleCard(31, "高畑勋", "日本", StyleFamilyT3, 2, 1, "A", "温柔写实", "淡雅写实 · 温和光 · 织物与器物细节 · 平视视角", "动作幅度小", StyleBase2DAnimation),
	styleCard(32, "奉俊昊", "韩国", StyleFamilyT2, 2, 1, "A", "类型混搭", "冷调 · 强场面调度 · 类型符号混搭 · 阶调统一", "暴力场面不等于动作语法"),
	styleCard(33, "朴赞郁", "韩国", StyleFamilyT2, 3, 2, "A", "惊悚暴力", "中轴对称 · 高对比 · 冷蓝与深红 · 定格式构图", "造作 / 仪式感过重"),
	styleCard(34, "李沧东", "韩国", StyleFamilyT5, 0, 0, "A", "社会写实", "粗粝手持 · 真实自然光 · 长镜 · 灰土调", "非动作向"),
	styleCard(35, "金炯柱", "韩国", StyleFamilyT4, 0, 1, "A", "时尚视觉", "高饱和撞色 · 装饰性构成 · 时尚造型感", "过时尚 / 脱离题材"),
	styleCard(36, "诺兰", "欧美", StyleFamilyT7, 2, 2, "A", "科幻动作", "巨物尺度 · 冷蓝高对比 · 中轴对称 · 大画幅 · 胶片高感", "动作偏重 / 节奏慢"),
	styleCard(37, "维伦纽瓦", "欧美", StyleFamilyT1, 2, 3, "A", "科幻史诗", "单色主导（橙/灰） · 巨大体量对比 · 低频构图 · 尘雾弥漫", "色调过灰 / 节奏拖沓"),
	styleCard(38, "塔伦蒂诺", "欧美", StyleFamilyT1, 3, 1, "A", "犯罪暴力", "极低机位仰拍 · 复古类型色 · 高反差 · 章节感", "时代错位 / 特效单薄"),
	styleCard(39, "大卫·芬奇", "欧美", StyleFamilyT1, 2, 2, "A", "悬疑犯罪", "冷绿/青 · 低照度 · 硬边光 · 数字锐度 · 秩序构图", "过冷过暗 / 打斗看不清"),
	styleCard(40, "韦斯·安德森", "欧美", StyleFamilyT2, 0, 0, "A", "喜剧美学", "正面中轴 · 粉彩低饱和 · 平面化 · 横移镜头 · 舞台式布景", "打斗变摆拍"),
	styleCard(41, "大卫·林奇", "欧美", StyleFamilyT2, 1, 1, "B", "超现实", "强色光源 · 广角变形 · 梦境空间 · 非自然配色", "非自然配色 / 观众出戏"),
	styleCard(42, "库布里克", "欧美", StyleFamilyT1, 3, 2, "C", "哲学惊悚", "绝对对称 · 冷暖极端分离 · 固定长镜", "AI难撑，须降级为固定对称"),
	styleCard(43, "泰伦斯·马利克", "欧美", StyleFamilyT6, 0, 1, "B", "诗电影", "自然光 · 极低机位 · 慢镜 · 长焦静观 · 黄昏", "无动作 / 节奏拖沓"),
	styleCard(44, "昆汀", "欧美", StyleFamilyT1, 3, 1, "A", "犯罪暴力", "低机位仰拍 · 章节字幕 · 九十年代犯罪质感", "特效单薄 / 时代错位"),
	styleCard(45, "索德伯格", "欧美", StyleFamilyT5, 1, 0, "A", "街拍犯罪", "冷调 · 自然光 · 轻手持 · 低对比 · 街拍感", "动作松散"),
	styleCard(46, "雷德利·斯科特", "欧美", StyleFamilyT7, 1, 2, "A", "古典史诗", "古典油画感 · 厚实暗部 · 暖金烛光 · 数字绘景全景", "打斗偏阵仗"),
	styleCard(47, "卡梅隆", "欧美", StyleFamilyT7, 2, 3, "A", "技术奇观", "巨物压迫 · 冷蓝 · 高动态范围 · 奇观场面", "重技术轻动作"),
	styleCard(48, "温子仁", "欧美", StyleFamilyT2, 2, 2, "B", "恐怖惊悚", "极低照度 · 幽闭空间 · 冷绿 · 局部强光 · 颤动", "太暗 / 动作丢失"),
	styleCard(49, "塔可夫斯基", "欧美", StyleFamilyT6, 0, 1, "C", "诗电影", "自然光 · 水与雾 · 极长镜 · 时间质感", "AI亦难撑，须拆静帧+慢镜"),
	styleCard(50, "帕索里尼", "欧美", StyleFamilyT6, 1, 1, "B", "古典油画", "极远景平视 · 黄昏低角度光 · 油画笔触", "人物过小 / 叙事断裂"),
	styleCard(51, "吕贝·贝松", "欧美", StyleFamilyT3, 1, 1, "A", "童年视角", "儿童视角低机位 · 暖自然光 · 静谧 · 浅景深", "不适合打斗"),
	styleCard(52, "罗杰·狄金斯", "摄影与视觉", StyleFamilyT2, 2, 2, "A", "实拍光影", "阴影侧取景 · 纯实用光源 · 冷暖分离 · 10~15cm 缓推", "禁无源夜戏蓝光 / 禁正面平光"),
	styleCard(53, "霍伊特·范·霍特", "摄影与视觉", StyleFamilyT5, 1, 0, "A", "纪录感", "手持 · 自然光 · 自然运动模糊 · 抓拍构图", "画面不稳 / 主体丢失"),
	styleCard(54, "牛顿·托马斯·西格尔", "摄影与视觉", StyleFamilyT3, 0, 1, "A", "胶片肖像", "大画幅 · 低反差 · 柔焦 · 胶片颗粒 · 中性色", "过于平缓 / 动作边界不清"),
	styleCard(55, "维托里奥·斯托拉罗", "摄影与视觉", StyleFamilyT4, 1, 2, "A", "象征用光", "象征性用光 · 色彩叙事 · 强反差光影 · 明暗块面", "构图优先于动作"),
	styleCard(56, "大流士·孔吉", "摄影与视觉", StyleFamilyT4, 1, 2, "A", "装饰光影", "粗颗粒 · 霓虹光晕 · 材质微距 · 装饰布光", "装饰压过叙事 / 材质过密"),
	styleCard(57, "荒木经惟", "摄影与视觉", StyleFamilyT5, 0, 1, "B", "侵入视角", "高 ISO 噪点 · 侵入式近距 · 弱光 · 冷绿日常", "噪点过量 / 人物变形"),
	styleCard(58, "安妮·莱博维茨", "摄影与视觉", StyleFamilyT4, 0, 1, "A", "明星肖像", "大画幅 · 硬朗布光 · 高反差 · 竖构图肖像", "静态肖像感 / 不适合连续动作"),
	styleCard(59, "艾曼努尔·卢贝兹基", "摄影与视觉", StyleFamilyT6, 1, 2, "C", "自然长镜", "自然光 · 一镜到底式长镜 · 手持跟随 · 魔幻时刻", "AI 难稳定长镜，须拆段"),
	styleCard(60, "罗伯特·理查德森", "摄影与视觉", StyleFamilyT1, 1, 2, "B", "激烈主观", "激烈手持 · 主观视角 · 高反差 · 混沌调度", "画面过乱 / 主体辨识下降"),
	styleCard(61, "斯文·尼奎斯特", "摄影与视觉", StyleFamilyT6, 0, 0, "B", "极简长镜", "自然光 · 柔和低反差 · 中远景 · 静止长拍", "无动作"),
	styleCard(62, "吉卜力", "动画与工作室", StyleFamilyT8, 2, 2, "A", "奇幻日常", "手绘水彩背景 · 柔和日光 · 淡雅彩 · 生活质感 · 云与风的写实", "偏飞行自然 / 非硬核打斗", StyleBase2DAnimation),
	styleCard(63, "新海诚", "动画与工作室", StyleFamilyT8, 1, 2, "A", "都市青春", "极致高饱和天空 · 强烈光斑 · 玻璃反射 · 都市青春", "过甜 / 人物失真", StyleBase2DAnimation),
	styleCard(64, "MAPPA", "动画与工作室", StyleFamilyT8, 3, 3, "A", "2D都市动画厚涂", "厚涂质感 · 高对比阴影 · 都市写实 · 动态变形帧", "日式赛璐璐 / 表情崩坏", StyleBase2DAnimation),
	styleCard(65, "国漫3D仙侠", "动画与工作室", StyleFamilyT8, 3, 3, "A", "国漫3D修仙玄幻", "次世代国漫3D半写实 · PBR丝绸金玉 · 丁达尔云雾 · 粒子流光剑气 · 7.5~8头身建模", "禁2D粗黑描边 / 禁廉价塑料反光", StyleBase3DCG),
	styleCard(66, "暗黑神话游戏CG", "动画与工作室", StyleFamilyT7, 3, 3, "A", "游戏CG暗黑神话", "虚幻5主机游戏CG · 高精PBR战损甲胄 · 暗黑高光比 · 丁达尔体积雾 · 火星余烬", "禁2D卡通平涂 / 禁轻飘无重力", StyleBase3DCG),
	styleCard(68, "押井守（动画向）", "动画与工作室", StyleFamilyT8, 2, 3, "B", "赛博动画", "冷调 · 玻璃幕墙 · 手绘人物+3D 背景混合", "混合质感不统一", StyleBase2DAnimation, StyleBase3DCG),
	styleCard(69, "罗小黑", "动画与工作室", StyleFamilyT8, 2, 2, "A", "2D中式水墨感", "2D中式水墨氛围 · 干净线条 · 留白 · 淡雅", "线条过粗 / 水墨退化为灰雾", StyleBase2DAnimation, StyleBaseInk2D),
	styleCard(71, "国漫3D进阶水墨", "动画与工作室", StyleFamilyT8, 3, 3, "A", "3D进阶水墨国风", "国漫3D三渲二水墨 · 枯笔飞白描边 · 狂草墨韵刀气 · 朱砂留白对冲 · 宣纸肌理", "禁退化为2D剪纸 / 禁死灰滤镜", StyleBase3DCG, StyleBaseInk2D),
	styleCard(72, "东方神话3D奇幻", "动画与工作室", StyleFamilyT7, 3, 3, "A", "东方神话3D", "东方神话3D电影感 · 织金玉石次表面散射 · 青绿赤金 · 巨物法相奇观", "禁2D粗黑边 / 禁阴暗废土风", StyleBase3DCG, StyleBaseInk2D),
	styleCard(73, "皮克斯", "动画与工作室", StyleFamilyT8, 2, 2, "A", "家庭向3D动画", "圆润造型 · 高饱和 · 柔和全局光 · 家庭向", "低幼感 / 动作力度不足", StyleBase3DCG),
	styleCard(74, "迪士尼经典", "动画与工作室", StyleFamilyT8, 1, 2, "A", "手绘赛璐璐", "手绘赛璐璐 · 粗描边 · 平涂色块 · 经典动画质感", "年代感过强 / 动作表现受限", StyleBase2DAnimation),
	styleCard(75, "皮埃尔·孔切 / 平面插画", "动画与工作室", StyleFamilyT4, 0, 1, "A", "平面插画", "平面插画 · 大色块 · 编辑设计感 · 有限色板", "空间纵深弱 / 不适合连续动作", StyleBase2DAnimation, StyleBaseInk2D),
	styleCard(76, "Nelagraphs / 长条漫画风", "动画与工作室", StyleFamilyT8, 1, 2, "A", "美式漫画", "美式漫画 · 粗黑描边 · 高对比网点 · 英雄姿态", "网点和粗描边压过细节", StyleBase2DAnimation, StyleBaseInk2D),
}

var (
	styleCardByID   = buildStyleCardIndex()
	styleFamilyByID = buildStyleFamilyIndex()
)

func styleCard(number int, name, group, family string, fight, effect int, grade, genre, prompt, warning string, compatibleBaseOverride ...string) StyleCard {
	bases := styleFamilyBaseDefaults[family]
	if len(compatibleBaseOverride) > 0 {
		bases = compatibleBaseOverride
	}
	return StyleCard{
		ID:                  styleCardID(number),
		Number:              number,
		Name:                name,
		Group:               group,
		Family:              family,
		FightRating:         fight,
		EffectRating:        effect,
		Grade:               grade,
		NativeGenre:         genre,
		PromptParameters:    prompt,
		AvoidanceWarning:    warning,
		CompatibleStyleBase: append([]string(nil), bases...),
	}
}

func styleCardID(number int) string {
	if number < 10 {
		return "sd-style-00" + string(rune('0'+number))
	}
	if number < 100 {
		return "sd-style-0" + string(rune('0'+number/10)) + string(rune('0'+number%10))
	}
	return ""
}

func buildStyleCardIndex() map[string]StyleCard {
	result := make(map[string]StyleCard, len(styleCards))
	for _, card := range styleCards {
		result[card.ID] = card
	}
	return result
}

func buildStyleFamilyIndex() map[string]StyleFamilyDefinition {
	result := make(map[string]StyleFamilyDefinition, len(styleFamilyDefinitions))
	for _, family := range styleFamilyDefinitions {
		result[family.ID] = family
	}
	return result
}

func (card StyleCard) SupportsStyleBase(styleBase string) bool {
	for _, candidate := range card.CompatibleStyleBase {
		if candidate == styleBase {
			return true
		}
	}
	return false
}

func StyleCatalog() StyleCatalogView {
	bases := append([]StyleBaseDefinition(nil), styleBaseDefinitions...)
	families := append([]StyleFamilyDefinition(nil), styleFamilyDefinitions...)
	cards := make([]StyleCard, 0, len(styleCards))
	for _, card := range styleCards {
		card.CompatibleStyleBase = append([]string(nil), card.CompatibleStyleBase...)
		cards = append(cards, card)
	}
	return StyleCatalogView{
		Version:  StyleCatalogVersion,
		Bases:    bases,
		Families: families,
		Items:    cards,
	}
}
