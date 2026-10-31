/* ============================================================
 * VisiDeep AI · 用户端门户
 * 对接 BeefTV /manju 管线。三种模式：
 *   mock  —— 纯前端生成示例三件套，无需后端（默认）
 *   demo  —— 仿真演示：浏览器内置与后端行为一致的 manju 状态机，
 *            P0→P5 全流程、四类人工确认门、媒体就绪、交付物入库
 *            全部可交互测试，无需真实后端
 *   real  —— 调用真实 /manju API 逐阶段运行：
 *            每阶段 run → 轮询 → 人工确认点 → 下一阶段
 *            全部工件进入「文件中心」，可查看 / 复制 / 下载
 * ============================================================ */

const $ = (s) => document.querySelector(s);
const $$ = (s) => Array.from(document.querySelectorAll(s));

/* ---------- 配置（持久化到 localStorage） ---------- */
const CONFIG = {
  mode: "mock",
  origin: "", // 留空 = 同源（服务器部署时走 Nginx 反代）；本地直连后端则填 http://127.0.0.1:8790
  token: "",
};

function loadConfig() {
  try {
    const saved = JSON.parse(localStorage.getItem("vd_config") || "{}");
    if (saved.mode) CONFIG.mode = saved.mode;
    if (saved.origin) CONFIG.origin = saved.origin;
    if (saved.token) CONFIG.token = saved.token;
  } catch {}
}
function saveConfig() {
  localStorage.setItem("vd_config", JSON.stringify(CONFIG));
}

/* ---------- 画风目录（内置快照；真实模式会从后端 /api/manju/style-catalog 覆盖） ---------- */
const BUILTIN_CATALOG = {
  version: "V7.4.26 (内置全量快照 · 74 张)",
  bases: [{"id": "live-action", "name": "真人实拍质感"}, {"id": "2d-animation", "name": "2D 手绘动画"}, {"id": "3d-cg", "name": "3D CG 动画"}, {"id": "ink-2d", "name": "水墨二维"}],
  families: {"T1": "刃光派", "T2": "冷峻秩序派", "T3": "柔光写实派", "T4": "浓艳装饰派", "T5": "纪实手持派", "T6": "极端写意派", "T7": "神话史诗派", "T8": "动画与国漫3D/CG派"},
  items: [
    {"id": "sd-style-001", "name": "张艺谋", "family": "T4", "feature": "仪式叙事", "promptParameters": "极高饱和色块 · 对称构图 · 大红/靛青对比 · 民艺纹样 · 群像横移 · 浓云硬光", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-002", "name": "陈凯歌", "family": "T7", "feature": "厚重史诗", "promptParameters": "巨景小人 · 逆光沙尘 · 金红与褐 · 极慢横移 · 沉重节奏", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-003", "name": "冯小刚", "family": "T3", "feature": "市民生活流", "promptParameters": "中焦对白 · 家居实景 · 冷蓝夜街与暖黄室内 · 快速对话切换", "compatible": ["live-action", "2d-animation"]},
    {"id": "sd-style-004", "name": "周星驰", "family": "T4", "feature": "港式喜剧", "promptParameters": "广角畸变 · 夸张表情 · 高饱和 · 快节奏 · 前景遮挡笑点", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-005", "name": "宁浩", "family": "T5", "feature": "类型拼贴", "promptParameters": "多段质感拼接 · 快剪 · 类型符号强化 · 章节感", "compatible": ["live-action"]},
    {"id": "sd-style-006", "name": "乌尔善", "family": "T7", "feature": "上古神话史诗", "promptParameters": "岩画兽纹 · 巨大体量对比 · 土红与骨白 · 篝火暖光 · 庄严缓推", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-007", "name": "路阳", "family": "T1", "feature": "武侠动作", "promptParameters": "冷青灰配烛火暖橙 · 低机位贴身跟拍 · 快刃三五合", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-008", "name": "毕赣", "family": "T6", "feature": "文艺诗电影", "promptParameters": "潮湿雾气 · 隧道霓虹 · 长焦压缩 · 时间失速", "compatible": ["live-action", "2d-animation", "ink-2d"]},
    {"id": "sd-style-009", "name": "徐克", "family": "T1", "feature": "武侠玄幻", "promptParameters": "高反差大光比 · 强轮廓硬光 · 冷蓝主调与暖金点光 · 体积雾 · 广角低机位仰拍 · 120fps 升格", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-010", "name": "王家卫", "family": "T2", "feature": "都市情感", "promptParameters": "抽帧残影 · 高饱和霓虹与暖黄交错 · 狭窄空间 · 贴身手持 · 前景遮挡 · 光晕渗化 · 慢门拖影", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-011", "name": "杜琪峰", "family": "T2", "feature": "都市写实", "promptParameters": "长焦压缩 · 冷静机位 · 疏离构图 · 灰蓝调 · 大留白", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-012", "name": "袁和平", "family": "T1", "feature": "武侠动作", "promptParameters": "兵器形制特写 · 布料与皮甲质感 · 真实重心与发力 · 汗与尘", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-013", "name": "彭顺", "family": "T2", "feature": "港式惊悚", "promptParameters": "逼仄空间 · 冷绿光 · 手持晃动 · 低照度 · 局部高光", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-014", "name": "陈可辛", "family": "T3", "feature": "都市情感", "promptParameters": "暖调柔光 · 生活实景 · 中近景 · 手持轻晃 · 情绪特写", "compatible": ["live-action", "2d-animation"]},
    {"id": "sd-style-015", "name": "李少红", "family": "T4", "feature": "古典唯美", "promptParameters": "柔焦 · 浓艳 · 绸缎光泽 · 对称帘幕 · 烛光暖影", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-016", "name": "刘镇伟", "family": "T4", "feature": "港式奇幻", "promptParameters": "高饱和奇幻 · 柔光人像 · 蓝紫幻光 · 星芒", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-017", "name": "李安", "family": "T3", "feature": "克制写实", "promptParameters": "自然侧光 · 中景为主 · 低饱和 · 留白 · 克制表演", "compatible": ["live-action", "2d-animation"]},
    {"id": "sd-style-018", "name": "侯孝贤", "family": "T5", "feature": "文艺写实", "promptParameters": "固定远景长镜 · 真实环境光 · 无戏剧化打光 · 平实构图", "compatible": ["live-action"]},
    {"id": "sd-style-019", "name": "杨德昌", "family": "T2", "feature": "都市疏离", "promptParameters": "中焦 · 冷青调 · 前后景深隔断 · 快速跳切", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-020", "name": "蔡明亮", "family": "T6", "feature": "极简写实", "promptParameters": "极低机位 · 手持 · 潮湿暗调 · 窗框遮挡 · 慢节奏", "compatible": ["live-action", "2d-animation", "ink-2d"]},
    {"id": "sd-style-021", "name": "王童", "family": "T4", "feature": "视觉华丽", "promptParameters": "民间装饰纹样 · 浓饱和 · 对称 · 图画性构图", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-022", "name": "黑泽明", "family": "T7", "feature": "时代剧", "promptParameters": "自然硬光 · 实景搭景 · 多机位群像 · 明快色彩 · 剪影构图", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-023", "name": "北野武", "family": "T6", "feature": "极简暴力", "promptParameters": "极简固定镜 · 大段空镜 · 海浪与风 · 骤停画格", "compatible": ["live-action", "2d-animation", "ink-2d"]},
    {"id": "sd-style-024", "name": "今村昌平", "family": "T5", "feature": "社会写实", "promptParameters": "人群挤压 · 自然光 · 土色系 · 群体动作混乱感", "compatible": ["live-action"]},
    {"id": "sd-style-025", "name": "是枝裕和", "family": "T3", "feature": "家庭日常", "promptParameters": "家庭实景 · 窗侧自然光 · 中近景 · 低对比 · 停顿", "compatible": ["live-action", "2d-animation"]},
    {"id": "sd-style-026", "name": "河濑直美", "family": "T3", "feature": "抒情", "promptParameters": "逆光柔焦 · 淡曝光 · 手持微晃 · 浅景深", "compatible": ["live-action", "2d-animation"]},
    {"id": "sd-style-027", "name": "岩井俊二", "family": "T3", "feature": "青春回忆", "promptParameters": "逆光光晕 · 柔焦 · 胶片青绿 · 慢节奏", "compatible": ["live-action", "2d-animation"]},
    {"id": "sd-style-028", "name": "押井守", "family": "T8", "feature": "赛博科幻", "promptParameters": "冷青 · 玻璃幕墙反射 · 长焦 · 静止构图 · 2D/3D 混合质感", "compatible": ["2d-animation", "3d-cg"]},
    {"id": "sd-style-029", "name": "大友克洋", "family": "T7", "feature": "机甲科幻", "promptParameters": "冷金属机械 · 微距 · 硬表面反射 · 精密结构", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-030", "name": "铃木敏郎", "family": "T3", "feature": "生活质感", "promptParameters": "水彩手绘 · 柔和日光 · 淡雅彩 · 生活器物", "compatible": ["2d-animation"]},
    {"id": "sd-style-031", "name": "高畑勋", "family": "T3", "feature": "温柔写实", "promptParameters": "淡雅写实 · 温和光 · 织物与器物细节 · 平视视角", "compatible": ["2d-animation"]},
    {"id": "sd-style-032", "name": "奉俊昊", "family": "T2", "feature": "类型混搭", "promptParameters": "冷调 · 强场面调度 · 类型符号混搭 · 阶调统一", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-033", "name": "朴赞郁", "family": "T2", "feature": "惊悚暴力", "promptParameters": "中轴对称 · 高对比 · 冷蓝与深红 · 定格式构图", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-034", "name": "李沧东", "family": "T5", "feature": "社会写实", "promptParameters": "粗粝手持 · 真实自然光 · 长镜 · 灰土调", "compatible": ["live-action"]},
    {"id": "sd-style-035", "name": "金炯柱", "family": "T4", "feature": "时尚视觉", "promptParameters": "高饱和撞色 · 装饰性构成 · 时尚造型感", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-036", "name": "诺兰", "family": "T7", "feature": "科幻动作", "promptParameters": "巨物尺度 · 冷蓝高对比 · 中轴对称 · 大画幅 · 胶片高感", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-037", "name": "维伦纽瓦", "family": "T1", "feature": "科幻史诗", "promptParameters": "单色主导（橙/灰） · 巨大体量对比 · 低频构图 · 尘雾弥漫", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-038", "name": "塔伦蒂诺", "family": "T1", "feature": "犯罪暴力", "promptParameters": "极低机位仰拍 · 复古类型色 · 高反差 · 章节感", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-039", "name": "大卫·芬奇", "family": "T1", "feature": "悬疑犯罪", "promptParameters": "冷绿/青 · 低照度 · 硬边光 · 数字锐度 · 秩序构图", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-040", "name": "韦斯·安德森", "family": "T2", "feature": "喜剧美学", "promptParameters": "正面中轴 · 粉彩低饱和 · 平面化 · 横移镜头 · 舞台式布景", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-041", "name": "大卫·林奇", "family": "T2", "feature": "超现实", "promptParameters": "强色光源 · 广角变形 · 梦境空间 · 非自然配色", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-042", "name": "库布里克", "family": "T1", "feature": "哲学惊悚", "promptParameters": "绝对对称 · 冷暖极端分离 · 固定长镜", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-043", "name": "泰伦斯·马利克", "family": "T6", "feature": "诗电影", "promptParameters": "自然光 · 极低机位 · 慢镜 · 长焦静观 · 黄昏", "compatible": ["live-action", "2d-animation", "ink-2d"]},
    {"id": "sd-style-044", "name": "昆汀", "family": "T1", "feature": "犯罪暴力", "promptParameters": "低机位仰拍 · 章节字幕 · 九十年代犯罪质感", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-045", "name": "索德伯格", "family": "T5", "feature": "街拍犯罪", "promptParameters": "冷调 · 自然光 · 轻手持 · 低对比 · 街拍感", "compatible": ["live-action"]},
    {"id": "sd-style-046", "name": "雷德利·斯科特", "family": "T7", "feature": "古典史诗", "promptParameters": "古典油画感 · 厚实暗部 · 暖金烛光 · 数字绘景全景", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-047", "name": "卡梅隆", "family": "T7", "feature": "技术奇观", "promptParameters": "巨物压迫 · 冷蓝 · 高动态范围 · 奇观场面", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-048", "name": "温子仁", "family": "T2", "feature": "恐怖惊悚", "promptParameters": "极低照度 · 幽闭空间 · 冷绿 · 局部强光 · 颤动", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-049", "name": "塔可夫斯基", "family": "T6", "feature": "诗电影", "promptParameters": "自然光 · 水与雾 · 极长镜 · 时间质感", "compatible": ["live-action", "2d-animation", "ink-2d"]},
    {"id": "sd-style-050", "name": "帕索里尼", "family": "T6", "feature": "古典油画", "promptParameters": "极远景平视 · 黄昏低角度光 · 油画笔触", "compatible": ["live-action", "2d-animation", "ink-2d"]},
    {"id": "sd-style-051", "name": "吕贝·贝松", "family": "T3", "feature": "童年视角", "promptParameters": "儿童视角低机位 · 暖自然光 · 静谧 · 浅景深", "compatible": ["live-action", "2d-animation"]},
    {"id": "sd-style-052", "name": "罗杰·狄金斯", "family": "T2", "feature": "实拍光影", "promptParameters": "阴影侧取景 · 纯实用光源 · 冷暖分离 · 10~15cm 缓推", "compatible": ["live-action", "3d-cg"]},
    {"id": "sd-style-053", "name": "霍伊特·范·霍特", "family": "T5", "feature": "纪录感", "promptParameters": "手持 · 自然光 · 自然运动模糊 · 抓拍构图", "compatible": ["live-action"]},
    {"id": "sd-style-054", "name": "牛顿·托马斯·西格尔", "family": "T3", "feature": "胶片肖像", "promptParameters": "大画幅 · 低反差 · 柔焦 · 胶片颗粒 · 中性色", "compatible": ["live-action", "2d-animation"]},
    {"id": "sd-style-055", "name": "维托里奥·斯托拉罗", "family": "T4", "feature": "象征用光", "promptParameters": "象征性用光 · 色彩叙事 · 强反差光影 · 明暗块面", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-056", "name": "大流士·孔吉", "family": "T4", "feature": "装饰光影", "promptParameters": "粗颗粒 · 霓虹光晕 · 材质微距 · 装饰布光", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-057", "name": "荒木经惟", "family": "T5", "feature": "侵入视角", "promptParameters": "高 ISO 噪点 · 侵入式近距 · 弱光 · 冷绿日常", "compatible": ["live-action"]},
    {"id": "sd-style-058", "name": "安妮·莱博维茨", "family": "T4", "feature": "明星肖像", "promptParameters": "大画幅 · 硬朗布光 · 高反差 · 竖构图肖像", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-059", "name": "艾曼努尔·卢贝兹基", "family": "T6", "feature": "自然长镜", "promptParameters": "自然光 · 一镜到底式长镜 · 手持跟随 · 魔幻时刻", "compatible": ["live-action", "2d-animation", "ink-2d"]},
    {"id": "sd-style-060", "name": "罗伯特·理查德森", "family": "T1", "feature": "激烈主观", "promptParameters": "激烈手持 · 主观视角 · 高反差 · 混沌调度", "compatible": ["live-action", "2d-animation", "3d-cg"]},
    {"id": "sd-style-061", "name": "斯文·尼奎斯特", "family": "T6", "feature": "极简长镜", "promptParameters": "自然光 · 柔和低反差 · 中远景 · 静止长拍", "compatible": ["live-action", "2d-animation", "ink-2d"]},
    {"id": "sd-style-062", "name": "吉卜力", "family": "T8", "feature": "奇幻日常", "promptParameters": "手绘水彩背景 · 柔和日光 · 淡雅彩 · 生活质感 · 云与风的写实", "compatible": ["2d-animation"]},
    {"id": "sd-style-063", "name": "新海诚", "family": "T8", "feature": "都市青春", "promptParameters": "极致高饱和天空 · 强烈光斑 · 玻璃反射 · 都市青春", "compatible": ["2d-animation"]},
    {"id": "sd-style-064", "name": "MAPPA", "family": "T8", "feature": "2D都市动画厚涂", "promptParameters": "厚涂质感 · 高对比阴影 · 都市写实 · 动态变形帧", "compatible": ["2d-animation"]},
    {"id": "sd-style-065", "name": "国漫3D仙侠", "family": "T8", "feature": "国漫3D修仙玄幻", "promptParameters": "次世代国漫3D半写实 · PBR丝绸金玉 · 丁达尔云雾 · 粒子流光剑气 · 7.5~8头身建模", "compatible": ["3d-cg"]},
    {"id": "sd-style-066", "name": "暗黑神话游戏CG", "family": "T7", "feature": "游戏CG暗黑神话", "promptParameters": "虚幻5主机游戏CG · 高精PBR战损甲胄 · 暗黑高光比 · 丁达尔体积雾 · 火星余烬", "compatible": ["3d-cg"]},
    {"id": "sd-style-068", "name": "押井守（动画向）", "family": "T8", "feature": "赛博动画", "promptParameters": "冷调 · 玻璃幕墙 · 手绘人物+3D 背景混合", "compatible": ["2d-animation", "3d-cg"]},
    {"id": "sd-style-069", "name": "罗小黑", "family": "T8", "feature": "2D中式水墨感", "promptParameters": "2D中式水墨氛围 · 干净线条 · 留白 · 淡雅", "compatible": ["2d-animation", "ink-2d"]},
    {"id": "sd-style-071", "name": "国漫3D进阶水墨", "family": "T8", "feature": "3D进阶水墨国风", "promptParameters": "国漫3D三渲二水墨 · 枯笔飞白描边 · 狂草墨韵刀气 · 朱砂留白对冲 · 宣纸肌理", "compatible": ["3d-cg", "ink-2d"]},
    {"id": "sd-style-072", "name": "东方神话3D奇幻", "family": "T7", "feature": "东方神话3D", "promptParameters": "东方神话3D电影感 · 织金玉石次表面散射 · 青绿赤金 · 巨物法相奇观", "compatible": ["3d-cg", "ink-2d"]},
    {"id": "sd-style-073", "name": "皮克斯", "family": "T8", "feature": "家庭向3D动画", "promptParameters": "圆润造型 · 高饱和 · 柔和全局光 · 家庭向", "compatible": ["3d-cg"]},
    {"id": "sd-style-074", "name": "迪士尼经典", "family": "T8", "feature": "手绘赛璐璐", "promptParameters": "手绘赛璐璐 · 粗描边 · 平涂色块 · 经典动画质感", "compatible": ["2d-animation"]},
    {"id": "sd-style-075", "name": "皮埃尔·孔切 / 平面插画", "family": "T4", "feature": "平面插画", "promptParameters": "平面插画 · 大色块 · 编辑设计感 · 有限色板", "compatible": ["2d-animation", "ink-2d"]},
    {"id": "sd-style-076", "name": "Nelagraphs / 长条漫画风", "family": "T8", "feature": "美式漫画", "promptParameters": "美式漫画 · 粗黑描边 · 高对比网点 · 英雄姿态", "compatible": ["2d-animation", "ink-2d"]},
  ],
};
let CATALOG = BUILTIN_CATALOG;

function isCardCompatible(cardId, base) {
  if (!base) return true;
  const it = (CATALOG.items || []).find((i) => i.id === cardId);
  if (!it) return true; // custom 等未登记项不拦
  return (it.compatible || []).includes(base);
}

// 联动过滤：根据当前选中的基底，只显示兼容的风格卡；不兼容项从源头不出现。
function populateStyleSelects(selectedBase) {
  const baseSel = $("#lockStyleBase");
  baseSel.innerHTML =
    '<option value="">请选择</option>' +
    CATALOG.bases.map((b) => `<option value="${b.id}">${b.name}</option>`).join("");
  if (selectedBase) baseSel.value = selectedBase;
  const base = selectedBase || baseSel.value;

  const cardSel = $("#lockStyleCard");
  const prev = cardSel.value;
  const groups = {};
  CATALOG.items.forEach((it) => {
    if (base && it.compatible && it.compatible.length && !it.compatible.includes(base)) return;
    const fam = CATALOG.families[it.family] || it.family;
    (groups[fam] = groups[fam] || []).push(it);
  });
  const famOrder = Object.keys(CATALOG.families); // T1..T8 顺序即展示顺序
  let opts = '<option value="">请选择风格卡</option>';
  famOrder.forEach((fid) => {
    const famName = CATALOG.families[fid];
    const list = groups[famName];
    if (!list || !list.length) return;
    opts += `<optgroup label="${famName}">`;
    opts += list
      .map((it) => `<option value="${it.id}">${it.name} · ${it.feature}</option>`)
      .join("");
    opts += "</optgroup>";
  });
  opts += '<option value="custom">自定义画风（下方描述）</option>';
  cardSel.innerHTML = opts;

  const becameIncompatible =
    prev && prev !== "custom" && !isCardCompatible(prev, base);
  if (prev && (prev === "custom" || isCardCompatible(prev, base))) {
    cardSel.value = prev;
  } else {
    cardSel.value = "";
  }
  refreshStyleHint();
  if (becameIncompatible) {
    const hint = $("#styleCardHint");
    hint.textContent = "已切换基底，原风格卡与该基底不兼容，已自动清空，请重选。";
    hint.classList.add("warn");
  }
}

async function ensureCatalog() {
  const prevBase = $("#lockStyleBase").value;
  if (CONFIG.mode !== "real") {
    CATALOG = BUILTIN_CATALOG;
    populateStyleSelects(prevBase);
    return;
  }
  try {
    const data = await api("/style-catalog", "GET");
    if (data && Array.isArray(data.bases) && Array.isArray(data.items)) {
      CATALOG = {
        version: data.version || "backend",
        bases: data.bases.map((b) => ({ id: b.id, name: b.name })),
        families: Object.fromEntries((data.families || []).map((f) => [f.id, f.name])),
        items: data.items.map((it) => ({
          id: it.id,
          name: it.name,
          family: it.family,
          feature: it.nativeGenre || it.genre || "",
          promptParameters: it.promptParameters || "",
          compatible: it.compatibleStyleBase || [],
        })),
      };
    }
  } catch (e) {
    CATALOG = BUILTIN_CATALOG;
  }
  populateStyleSelects(prevBase);
}

function refreshStyleHint() {
  const cardSel = $("#lockStyleCard");
  const wrap = $("#styleCustomWrap");
  const hint = $("#styleCardHint");
  const v = cardSel.value;
  wrap.style.display = v === "custom" ? "block" : "none";
  if (v === "custom") {
    hint.textContent = "自定义：在下方填写你自己的视觉参数，后端会据此生成风格块并锁死全片。";
    hint.classList.remove("warn");
    return;
  }
  const it = (CATALOG.items || []).find((i) => i.id === v);
  if (!it) { hint.textContent = ""; return; }
  const base = (CATALOG.bases || []).find((b) => b.id === $("#lockStyleBase").value);
  let msg = `已选：${it.name}（${CATALOG.families[it.family] || it.family}）｜ ${it.feature}\n视觉参数：${it.promptParameters}`;
  if (base && it.compatible && it.compatible.length && !it.compatible.includes(base.id)) {
    msg += `\n⚠️ 当前基底「${base.name}」与该风格卡不兼容，后端会拒绝（建议选 ${it.compatible.join(" / ")}）。`;
    hint.classList.add("warn");
  } else {
    hint.classList.remove("warn");
  }
  hint.textContent = msg;
}

function styleLabel(l) {
  if (l.styleCardId === "custom") return "自定义画风";
  const it = (CATALOG.items || []).find((i) => i.id === l.styleCardId);
  const base = (CATALOG.bases || []).find((b) => b.id === l.styleBase);
  return (it ? it.name : l.styleCardId) + (base ? " / " + base.name : "");
}

function styleNoteOf(l) {
  if (l.styleCardId === "custom") return `自定义画风：${l.styleCustomDescription || ""}`;
  const it = (CATALOG.items || []).find((i) => i.id === l.styleCardId);
  if (!it) return "系统默认美学";
  return `${it.name} 风格｜${it.promptParameters}`;
}

/* ---------- 中文选项 → 后端规范值映射（BeefTV service.go 枚举） ---------- */
const TRACK_MAP = { "①国内剧": "domestic", "②海外剧": "overseas", "③解说叙事类": "explainer", "④其他": "other" };
const SOURCE_MAP = { "①原创": "original", "②改编有原著": "adaptation", "③对标爆款": "benchmark", "④热点速成": "trend" };
const MODEL_MAP = { "Seedance 2.5": "seedance-2.5", "Seedance 2.0": "seedance-2.0", "MiniMax H3": "minimax-h3" };
const ASPECT_MAP = { "16:9 横屏": "16:9", "9:16 竖屏": "9:16", "21:9 宽幕": "21:9" };
const DELIVERY_MAP = { "①只要看板": "board", "②只要 md": "markdown", "③都要": "both" };
const SEAM_MAP = { "①末帧定格": "tail-frame-freeze", "②换机位硬切": "hard-cut", "③空镜": "insert-shot", "④技巧转场": "creative-transition" };
const MATERIAL_MAP = { "M0 纯提示词": "M0", "M1 编号占位(默认)": "M1", "M2 已入库": "M2" };

const REQUIRED_LOCKS = [
  ["lockTrack", "赛道"], ["lockSource", "素材来源"], ["lockModel", "模型版本"],
  ["lockAspect", "画幅"], ["lockDuration", "目标时长"], ["lockDelivery", "交付形态"],
  ["lockSeam", "段缝衔接"], ["lockStyleBase", "画风基底"], ["lockStyleCard", "风格卡"],
  ["lockMaterial", "素材形态"],
];

function collectLocks() {
  const missing = REQUIRED_LOCKS.filter(([id]) => {
    const el = $("#" + id);
    return !el.value || (id === "lockDuration" && Number(el.value) <= 0);
  });
  if (missing.length) {
    $("#lockHint").textContent = "请补全：" + missing.map(([, n]) => n).join("、");
    return null;
  }
  const brief = $("#brief").value.trim();
  if (!brief) {
    $("#lockHint").textContent = "请填写创意简述 / 剧本梗概";
    return null;
  }
  const card = $("#lockStyleCard").value;
  const customDesc = card === "custom" ? $("#styleCustom").value.trim() : "";
  if (card === "custom" && !customDesc) {
    $("#lockHint").textContent = "风格卡选了「自定义」，请填写自定义画风描述";
    return null;
  }
  const sourceLabel = $("#lockSource").value;
  const locks = {
    track: TRACK_MAP[$("#lockTrack").value] || $("#lockTrack").value,
    source: SOURCE_MAP[sourceLabel] || sourceLabel,
    videoModel: MODEL_MAP[$("#lockModel").value] || $("#lockModel").value,
    aspectRatio: ASPECT_MAP[$("#lockAspect").value] || $("#lockAspect").value,
    deliveryMode: DELIVERY_MAP[$("#lockDelivery").value] || $("#lockDelivery").value,
    seamMode: SEAM_MAP[$("#lockSeam").value] || $("#lockSeam").value,
    targetDurationSeconds: Math.max(5, Math.min(600, Number($("#lockDuration").value) || 60)),
    styleBase: $("#lockStyleBase").value,
    styleCardId: card,
    styleCustomDescription: customDesc,
    assetMode: MATERIAL_MAP[$("#lockMaterial").value] || $("#lockMaterial").value,
    brief,
    // 展示用标签（不入后端锁）
    _labels: {
      track: $("#lockTrack").value, source: sourceLabel, model: $("#lockModel").value,
      aspect: $("#lockAspect").value, delivery: $("#lockDelivery").value, seam: $("#lockSeam").value,
    },
  };
  // P0 硬锁逐项确认（后端 validateConfirmedLocks 要求）
  locks.confirmedLocks = ["track", "source", "videoModel", "aspectRatio", "deliveryMode", "styleCard", "seamMode"];
  // 热点速成需要执行口径
  if (locks.source === "trend") {
    const tr = $("#lockTrendResolution").value;
    if (!tr) {
      $("#lockHint").textContent = "热点速成项目必须选择「按原创」或「按对标」执行";
      return null;
    }
    locks.trendResolution = tr;
  }
  return locks;
}

/* ============================================================
 * 文件中心 Vault：所有阶段产物入库（localStorage），可查看/下载
 * ============================================================ */
const Vault = {
  key(projectId) { return "vd_files_" + projectId; },
  list(projectId) {
    try { return JSON.parse(localStorage.getItem(this.key(projectId)) || "[]"); }
    catch { return []; }
  },
  upsert(projectId, items) {
    if (!projectId || !items || !items.length) return;
    const all = this.list(projectId);
    const idx = new Map(all.map((f, i) => [f.uid, i]));
    for (const it of items) {
      if (!it.uid) continue;
      const row = {
        uid: it.uid, phase: it.phase || "阶段产物", stage: it.stage || "",
        kind: it.kind || "", version: it.version || 1,
        name: it.name || it.kind || "未命名.md",
        mime: it.mime || "text/markdown; charset=utf-8",
        content: it.content || "", createdAt: it.createdAt || new Date().toISOString(),
      };
      if (idx.has(row.uid)) all[idx.get(row.uid)] = row;
      else { all.push(row); idx.set(row.uid, all.length - 1); }
    }
    try {
      localStorage.setItem(this.key(projectId), JSON.stringify(all));
    } catch (e) {
      // 超出配额：只保留最近 60 份
      all.sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1));
      localStorage.setItem(this.key(projectId), JSON.stringify(all.slice(0, 60)));
    }
  },
  clear(projectId) { localStorage.removeItem(this.key(projectId)); },
};

function artifactToFile(phase, stageTitle, art) {
  const kind = art.kind || "artifact";
  const ext = kind.includes(".") ? "" : ".md";
  const name = `${phase}-${stageTitle || kind}-v${art.version || 1}-${kind}${ext}`;
  return {
    uid: `${phase}:${art.id || (art.stageId + ":" + art.version)}`,
    phase, stage: stageTitle, kind, version: art.version || 1,
    name, mime: "text/markdown; charset=utf-8",
    content: art.content || "", createdAt: art.createdAt || new Date().toISOString(),
  };
}

function deliverableToFile(d) {
  return {
    uid: `P5交付:${d.id}`,
    phase: "P5 最终交付", stage: d.kind, kind: d.kind, version: 1,
    name: d.fileName || d.kind, mime: d.mimeType || "text/markdown; charset=utf-8",
    content: d.content || "", createdAt: d.createdAt || new Date().toISOString(),
  };
}

/* ---------- 文件中心 UI ---------- */
function renderFiles() {
  const list = $("#fileList");
  const projectId = ENGINE.projectId;
  if (!projectId) { list.innerHTML = '<p class="hint">尚无运行项目。</p>'; return; }
  const files = Vault.list(projectId);
  if (!files.length) {
    list.innerHTML = '<p class="hint">还没有产物。开始运行后，每个阶段的文件会自动出现在这里。</p>';
    return;
  }
  const groups = {};
  files.forEach((f) => (groups[f.phase] = groups[f.phase] || []).push(f));
  const phaseOrder = ["P1 剧本", "P2 数字资产", "P0B 决策", "P3 分镜与故事板", "P4 投喂", "P5 最终交付", "示例交付"];
  const keys = Object.keys(groups).sort(
    (a, b) => (phaseOrder.indexOf(a) + 99) % 99 - ((phaseOrder.indexOf(b) + 99) % 99)
  );
  list.innerHTML = keys.map((ph) => `
    <div class="file-group">
      <div class="fg-title">${escHtml(ph)} <span class="fg-count">${groups[ph].length} 份</span></div>
      ${groups[ph].map((f) => `
        <div class="file-row" data-uid="${escAttr(f.uid)}">
          <span class="file-ico">📄</span>
          <div class="file-main">
            <div class="file-name">${escHtml(f.name)}</div>
            <div class="file-meta">${escHtml(f.stage || f.kind)} · v${f.version} · ${formatBytes((f.content || "").length)} · ${new Date(f.createdAt).toLocaleString("zh-CN")}</div>
          </div>
          <div class="file-acts">
            <button class="mini-btn" data-view="${escAttr(f.uid)}">查看</button>
            <button class="mini-btn" data-copyuid="${escAttr(f.uid)}">复制</button>
            <button class="mini-btn" data-dluid="${escAttr(f.uid)}">下载</button>
          </div>
        </div>`).join("")}
    </div>`).join("");

  list.querySelectorAll("[data-view]").forEach((b) =>
    b.addEventListener("click", () => openViewer(b.dataset.view)));
  list.querySelectorAll("[data-copyuid]").forEach((b) =>
    b.addEventListener("click", () => {
      const f = Vault.list(ENGINE.projectId).find((x) => x.uid === b.dataset.copyuid);
      if (f) copyText(f.content);
    }));
  list.querySelectorAll("[data-dluid]").forEach((b) =>
    b.addEventListener("click", () => {
      const f = Vault.list(ENGINE.projectId).find((x) => x.uid === b.dataset.dluid);
      if (f) downloadBlob(f.name, f.content, f.mime);
    }));
}

let fvCurrent = null;
/* 文件中心实时刷新：仅在产物数量变化时重渲染，避免打断查看/滚动 */
let _lastFileCount = -1;
function syncFilesView(force) {
  const n = ENGINE.projectId ? Vault.list(ENGINE.projectId).length : 0;
  const fab = $("#fabFiles");
  if (n > 0) { $("#fabCount").textContent = n; fab.classList.remove("hidden"); }
  else fab.classList.add("hidden");
  if (force || n !== _lastFileCount) {
    _lastFileCount = n;
    if (!$("#stepFiles").classList.contains("hidden")) renderFiles();
  }
}
function openViewer(uid) {
  const f = Vault.list(ENGINE.projectId).find((x) => x.uid === uid);
  if (!f) return;
  fvCurrent = f;
  $("#fvName").textContent = f.name;
  $("#fvMeta").textContent = `${f.phase} · ${f.stage || ""} · v${f.version} · ${formatBytes((f.content || "").length)}`;
  const body = $("#fvBody");
  if ((f.mime || "").includes("text/html")) {
    body.innerHTML = `<iframe class="fv-frame" sandbox=""></iframe>`;
    body.querySelector("iframe").srcdoc = f.content;
  } else {
    const pre = document.createElement("pre");
    pre.className = "fv-pre";
    pre.textContent = f.content || "（空内容）";
    body.innerHTML = "";
    body.appendChild(pre);
  }
  $("#fileViewer").classList.remove("hidden");
}
function closeViewer() {
  $("#fileViewer").classList.add("hidden");
  $("#fvBody").innerHTML = "";
  fvCurrent = null;
}

function downloadBlob(name, text, mime) {
  const blob = new Blob([text], { type: (mime || "text/markdown") + ";charset=utf-8" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = name.replace(/[\\/:*?"<>|]/g, "-");
  a.click();
  URL.revokeObjectURL(a.href);
}

function downloadAll() {
  const files = Vault.list(ENGINE.projectId || "");
  if (!files.length) { $("#filesHint").textContent = "暂无可下载的文件"; return; }
  const head = `# VisiDeep AI · 全阶段产物归档\n\n项目：${ENGINE.projectId || "-"}\n导出时间：${new Date().toLocaleString("zh-CN")}\n文件数：${files.length}\n\n---\n`;
  const body = files.map((f) => `\n\n<!-- ==== ${f.phase} / ${f.name} ==== -->\n# ${f.phase} · ${f.name}\n\n${f.content}\n`).join("\n---\n");
  downloadBlob(`VisiDeep-全阶段产物-${(ENGINE.projectId || "archive")}.md`, head + body, "text/markdown");
  $("#filesHint").textContent = `已打包 ${files.length} 份文件`;
}

/* ============================================================
 * API（解包后端 {code,data,msg} 信封）
 * ============================================================ */
async function api(path, method, body) {
  // 仿真演示模式：请求不发出，交给内置状态机处理（与后端契约一致）
  if (CONFIG.mode === "demo") return unwrapItem(await SimAPI.handle(method, path, body || null));
  const headers = { "Content-Type": "application/json" };
  if (CONFIG.token) headers["Authorization"] = "Bearer " + CONFIG.token;
  let res;
  try {
    res = await fetch(CONFIG.origin + "/api/manju" + path, {
      method, headers, body: body ? JSON.stringify(body) : undefined,
    });
  } catch (netErr) {
    const e = new Error("NETWORK_FAIL");
    e.kind = "network";
    e.detail = netErr;
    throw e;
  }
  const text = await res.text();
  let payload = {};
  try { payload = JSON.parse(text); } catch {}
  if (!res.ok) {
    const e = new Error(payload.msg || payload.message || payload.error || "HTTP " + res.status);
    e.kind = "http";
    e.status = res.status;
    e.reason = payload.reason || "";
    e.resBody = payload;
    throw e;
  }
  return payload && payload.data !== undefined ? unwrapItem(payload.data) : payload;
}

// 单项端点返回 {item: detail}，自动解包；列表 {items} 与目录保持原样
function unwrapItem(data) {
  if (
    data && typeof data === "object" && !Array.isArray(data) &&
    Object.keys(data).length === 1 && data.item !== undefined
  ) return data.item;
  return data;
}

/* ============================================================
 * 仿真演示后端（demo 模式）：浏览器内置 manju 状态机
 * 与 BeefTV 后端的阶段/状态/门控行为保持一致，用于无后端完整测试：
 *   P1 六阶段 + 圆桌决策门（可走精修重写支线）
 *   → P2 三阶段（清单确认门 / 交付确认门）
 *   → P0B 锚点门 → P3（故事板图片就绪门）→ P4（预演+视频就绪门）
 *   → P5 质检门 → 交付物入库（md + 看板 html）
 * 同样支持错误路径：状态冲突返回 409，可直接测试报错文案。
 * ============================================================ */
const SIM_STAGE_MS = 2600;   // 每个「生成中」阶段的耗时
const SIM_MEDIA_MS = 1900;   // 图片/视频逐张就绪的间隔

const SimAPI = (() => {
  const seq = { n: 0 };
  const id = (p) => p + "-" + (++seq.n).toString(36) + Math.random().toString(36).slice(2, 6);
  const db = { locks: {}, runs: {}, deliverables: {} };
  const now = () => Date.now();
  const iso = () => new Date().toISOString();

  function err(status, msg) {
    const e = new Error(msg);
    e.kind = "http";
    e.status = status;
    throw e;
  }
  const SCRIPT_DEFS = [
    ["source_registration", "来源登记"],
    ["ip_development", "IP 与角色开发"],
    ["series_bible", "全剧开发圣经"],
    ["episode_planning", "分集规划"],
    ["script_draft", "正式剧本"],
    ["roundtable_review", "五席圆桌审核"],
  ];
  const SCRIPT_DEFS_BENCH = [["recon_intelligence", "对标情报卡"], ...SCRIPT_DEFS];
  const POLISH_DEFS = [
    ["polish_draft", "补表演层与调度层"],
    ["polish_findings", "润色发现清单"],
    ["polish_confirmation", "润色回签"],
  ];
  const ASSET_DEFS = [["extraction", "资产提取"], ["manifest", "资产清单"], ["compile", "资产交付编译"]];
  const PROD_DEFS = [["decision_anchor", "P0B 决策锚点"], ["storyboard", "P3 分镜与故事板"], ["feed", "P4 投喂稿"], ["qc", "P5 质检与交付"]];

  function mkStages(defs) {
    return defs.map(([key, title]) => ({
      id: id("stg"), key, title,
      status: "NOT_STARTED", version: 0, artifactId: "",
      readyAt: 0, reviewGrade: "", draft: null, confirmedAt: "",
    }));
  }
  function startStage(r, st, delay) {
    st.status = "IN_PROGRESS";
    st.readyAt = now() + (delay == null ? SIM_STAGE_MS : delay);
    r.run.currentStage = st.key;
  }
  function findRun(pid, kind, rid) {
    const r = db.runs[rid];
    if (!r || r.projectId !== pid || r.kind !== kind) err(404, "运行任务不存在（或已过期）");
    return r;
  }
  function findStage(r, key) {
    const st = r.stages.find((x) => x.key === key);
    if (!st) err(404, "阶段不存在：" + key);
    return st;
  }
  function attachArtifact(r, st) {
    st.version += 1;
    const art = {
      id: id("art"), stageId: st.id, kind: st.key + ".md", version: st.version,
      content: simContent(r.locks, st.key, st.title), createdAt: iso(),
    };
    r.artifacts.push(art);
    st.artifactId = art.id;
    return art;
  }
  function advanceScript(r) {
    for (const st of r.stages) {
      if (st.status === "IN_PROGRESS" && now() >= st.readyAt) {
        attachArtifact(r, st);
        if (st.key === "roundtable_review") {
          st.status = "AWAITING_USER_DECISION";
          st.reviewGrade = "A";
          st.draft = { content: simContent(r.locks, st.key, st.title) };
        } else {
          st.status = "AWAITING_CONFIRMATION";
        }
      }
    }
  }
  function lockAfter(r, st) {
    st.status = "LOCKED";
    st.confirmedAt = iso();
    const next = r.stages.find((x) => x.status === "NOT_STARTED");
    if (next) startStage(r, next);
    else { r.run.status = "COMPLETED"; r.run.currentStage = ""; }
  }
  function scriptDetail(r) {
    advanceScript(r);
    return { run: { ...r.run }, stages: r.stages.map((s) => ({ ...s, readyAt: undefined })), artifacts: r.artifacts, roundtableDecisions: [] };
  }
  function advanceAsset(r) {
    for (const st of r.stages) {
      if (st.status === "IN_PROGRESS" && now() >= st.readyAt) {
        attachArtifact(r, st);
        if (st.key === "extraction") {
          // 与后端一致：提取完成自动锁定，清单直接进入待确认
          st.status = "LOCKED";
          st.confirmedAt = iso();
          const m = findStage(r, "manifest");
          attachArtifact(r, m);
          m.status = "AWAITING_CONFIRMATION";
        } else {
          st.status = "AWAITING_CONFIRMATION";
        }
      }
    }
  }
  function assetDetail(r) {
    advanceAsset(r);
    return { run: { ...r.run }, stages: r.stages.map((s) => ({ ...s, readyAt: undefined })), artifacts: r.artifacts, references: [] };
  }
  function advanceMedia(r) {
    r.images.forEach((i) => { if (i.status === "GENERATING" && now() >= i.readyAt) i.status = "READY"; });
    r.videos.forEach((v) => { if (v.status === "GENERATING" && now() >= v.readyAt) v.status = "READY"; });
    if (r.previs && r.previs.status === "RUNNING" && now() >= r.previs.readyAt) r.previs.status = "READY";
  }
  function advanceProduction(r) {
    advanceMedia(r);
    for (const st of r.stages) {
      if (st.status === "IN_PROGRESS" && now() >= st.readyAt) {
        attachArtifact(r, st);
        st.status = "AWAITING_CONFIRMATION";
      }
    }
  }
  function prodDetail(r) {
    advanceProduction(r);
    return {
      run: { ...r.run }, stages: r.stages.map((s) => ({ ...s, readyAt: undefined })), artifacts: r.artifacts,
      images: r.images, videos: r.videos, previs: r.previs,
    };
  }
  function buildDeliverables(r) {
    const L = r.locks || {};
    const md = `# 最终交付汇总（仿真演示）\n\n> 模型 ${L.videoModel || "-"} · 画幅 ${L.aspectRatio || "-"} · 交付形态 ${L.deliveryMode || "-"}\n\n${r.artifacts.map((a) => `## ${a.kind}\n\n${a.content}\n`).join("\n---\n\n")}`;
    const board = `<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><title>分镜看板（仿真）</title>
<style>body{font-family:system-ui;background:#f6f7fb;color:#1c2333;padding:24px}h1{font-size:20px}.card{background:#fff;border-radius:10px;padding:14px 16px;margin:10px 0;box-shadow:0 1px 4px rgba(0,0,0,.08)}.seg{font-size:13px;color:#5a6478}</style></head><body>
<h1>分镜看板 · EP1（仿真演示）</h1>
<div class="card"><b>镜1</b> 近景 · 沈昭雨夜睁眼<div class="seg">0-3s · 台词：（画外）这一世，我不会再信你。</div></div>
<div class="card"><b>镜2</b> 中景 · 林婉伪善递酒<div class="seg">3-7s · 台词：妹妹，喝了吧，暖暖身子。</div></div>
<div class="card"><b>镜3</b> 特写 · 怀表落地<div class="seg">7-11s · 无声 · 表盖弹开</div></div>
<div class="card"><b>镜4</b> 全景 · 落地窗前立誓<div class="seg">11-15s · 台词：（画外）游戏，才刚开始。</div></div>
</body></html>`;
    db.deliverables[r.projectId] = [
      { id: id("dlv"), fileName: "最终交付汇总.md", mimeType: "text/markdown; charset=utf-8", content: md, createdAt: iso() },
      { id: id("dlv"), fileName: "分镜看板.html", mimeType: "text/html; charset=utf-8", content: board, createdAt: iso() },
    ];
  }
  function simContent(L, key, title) {
    L = L || {};
    const style = L.styleCardId === "custom" ? "自定义画风" : (L.styleCardId || "-");
    const head = `# ${title}\n\n> 模型 ${L.videoModel || "-"} · 画幅 ${L.aspectRatio || "-"} · 风格 ${style} · 段缝 ${L.seamMode || "-"}\n`;
    const bodies = {
      source_registration: `## 登记信息\n- 素材来源：${L.source || "original"}\n- 授权范围：仅本项目使用\n\n## 登记梗概\n\n${(L.brief || "（未提供梗概）").slice(0, 400)}`,
      recon_intelligence: `## 对标情报卡\n- 对标类型：豪门重生\n- 爆款要素：3 秒重生钩子 · 打脸反转 · 情绪卡点\n- 结论：沿用七段式投喂结构`,
      ip_development: `## 主角\n- 沈昭（暂定）：25 岁，重生复仇，冷外热内。\n\n## 反派\n- 林婉（暂定）：伪善堂姐，珍珠耳环。\n\n## 核心冲突\n重生 + 悬念 + 复仇三段式。`,
      series_bible: `## 世界观\n现代都市豪门。\n\n## 基调\n短剧爽感 + 悬疑钩子。\n\n## 角色圣经\n（仿真占位：全剧人物弧线与关系网）`,
      episode_planning: `## 分集规划（目标 ${L.targetDurationSeconds || 60}s）\n- EP1 重生睁眼 · 钩子：怀表停在死亡时刻\n- EP2 伪善递酒 · 反转\n- EP3 立誓复仇 · 卡点`,
      script_draft: `## 第 1 集\n\n【场 1】夜·车内\n沈昭：（猛然睁眼）这一世……我不会再信你。\n\n【场 2】夜·豪宅客厅\n林婉：妹妹，喝了吧，暖暖身子。\n\n（仿真占位：正式剧本按 STEPWISE 支线逐集产出）`,
      roundtable_review: `# 五席圆桌审核\n\n- 剧作席：钩子成立，节奏合格。\n- 观众席：前 3 秒抓人。\n- 合规席：无风险点。\n- 制作席：可进入资产链。\n- 运营席：建议 EP2 增加反转。\n\n**综合等级：A**`,
      polish_draft: `## 补表演层\n- 为主要对手戏补微表情与停顿。\n\n## 补调度层\n- 递酒一场改为双人物纵深走位。`,
      polish_findings: `## 润色发现清单\n1. EP1 台词口语化\n2. 段缝预留定格帧\n3. 反派出场提前半拍`,
      polish_confirmation: `# 润色回签\n\n以上润色项已全部落稿，等待制片确认回签。`,
      extraction: `## 资产提取（${L.assetMode || "M1"}）\n\n### 角色\n- 沈昭 · 提示词：冷调侧光，高对比，情绪克制。\n### 场景\n- 豪宅客厅（夜）· 水晶灯，雨夜落地窗。\n### 道具\n- 旧怀表 · 微距，做旧黄铜。`,
      manifest: `## 资产清单\n\n| 编号 | 资产 | 形态 |\n|---|---|---|\n| M1 | 沈昭 | 角色 |\n| M2 | 林婉 | 角色 |\n| M3 | 豪宅客厅 | 场景 |\n| M4 | 旧怀表 | 道具 |`,
      compile: `## 资产交付汇编（${L.assetMode || "M1"}）\n\n（按素材形态编译的完整接线单，仿真占位）`,
      decision_anchor: `# P0B 决策锚点\n\n- 分集：EP1 · 镜头数：4\n- 时长分配：3s / 4s / 4s / 4s\n- 段缝衔接：${L.seamMode || "-"}\n- 模型：${L.videoModel || "-"}`,
      storyboard: `# P3 分镜与故事板\n\n【镜1】近景·雨夜睁眼 · 提示词：车窗冷蓝反光，湿发贴额，惊惶转清醒。\n【镜2】中景·伪善递酒 · 提示词：暖厅冷人对比，浅景深，慢推。\n【镜3】特写·怀表落地 · 提示词：微距慢镜，黄铜反光，尘埃浮动。\n【镜4】全景·立誓剪影 · 提示词：城市霓虹勾勒轮廓，史诗感。`,
      feed: `# P4 投喂稿（七段式）\n\n【核心人物】沈昭（清冷→隐忍）、林婉（甜腻→阴冷）\n\n【镜头1】近景 0-3s · 台词：（画外）这一世，我不会再信你。\n【镜头2】中景 3-7s · 台词：妹妹，喝了吧。\n【镜头3】特写 7-11s · 无声·表盖弹开\n【镜头4】全景 11-15s · 台词：游戏，才刚开始。\n\n【段缝】${L.seamMode || "-"} 衔接至下一段`,
      qc: `# P5 质检与最终交付\n\n- 全部镜头契约校验通过 ✓\n- 段缝一致性 ✓\n- 台词/时长配比 ✓\n- 交付形态：${L.deliveryMode || "-"}`,
    };
    return head + (bodies[key] || `## ${title}\n\n（仿真占位内容）`);
  }

  /* ---------- 路由 ---------- */
  async function handle(method, path, body) {
    const s = path.split("?")[0].split("/").filter(Boolean);
    if (s[0] !== "projects") err(404, "仿真后端未知端点：" + method + " " + path);
    const pid = s[1] || "";
    if (method === "PUT" && s[2] === "lock") {
      if (!body || !body.track) err(400, "立项锁参数无效");
      db.locks[pid] = body;
      return { ok: true };
    }
    if (s[2] === "script-runs") return scriptRoutes(pid, method, s, body);
    if (s[2] === "asset-runs") return assetRoutes(pid, method, s, body);
    if (s[2] === "production-runs") return prodRoutes(pid, method, s, body);
    if (s[2] === "deliverables" && method === "GET") {
      const items = db.deliverables[pid];
      if (!items) err(404, "交付物尚未生成：请先完成 P5 质检锁定");
      return { items };
    }
    err(404, "仿真后端未知端点：" + method + " " + path);
  }

  function scriptRoutes(pid, method, s, body) {
    if (method === "POST" && !s[3]) {
      const L = db.locks[pid];
      if (!L) err(409, "请先提交立项锁");
      const branch = L.source === "adaptation" ? "adaptation" : L.source === "benchmark" ? "benchmark" : "original";
      const defs = branch === "benchmark" ? SCRIPT_DEFS_BENCH : SCRIPT_DEFS;
      const r = { projectId: pid, kind: "script", locks: L, stages: mkStages(defs), artifacts: [], images: [], videos: [], previs: null };
      r.run = { id: id("srun"), status: "ACTIVE", branch, assetMode: L.assetMode || "M1", currentStage: "" };
      db.runs[r.run.id] = r;
      startStage(r, r.stages[0]);
      return scriptDetail(r);
    }
    const r = findRun(pid, "script", s[3]);
    if (method === "GET" && !s[4]) return scriptDetail(r);
    if (method === "POST" && s[4] === "source") {
      const st = findStage(r, "source_registration");
      // 与真实后端一致：阶段生成中即可登记来源（作为该阶段的输入）
      if (st.status !== "IN_PROGRESS" && !st.artifactId)
        err(409, "来源登记阶段未在生成中，无法登记");
      r.source = body || {};
      return { ok: true };
    }
    if (method === "POST" && s[4] === "stages") {
      const key = s[5], op = s[6];
      const st = findStage(r, key);
      if (op === "run") {
        if (st.status !== "NOT_STARTED" && st.status !== "BLOCKED" && st.status !== "STALE")
          err(409, `「${st.title}」当前状态为 ${st.status}，不可执行`);
        startStage(r, st);
        return scriptDetail(r);
      }
      if (op === "confirm") {
        if (st.status !== "AWAITING_CONFIRMATION" || !st.artifactId)
          err(409, `「${st.title}」尚未生成完成，不可确认`);
        lockAfter(r, st);
        return scriptDetail(r);
      }
      if (op === "decide") {
        if (st.status !== "AWAITING_USER_DECISION")
          err(409, `「${st.title}」当前不在待决策状态`);
        const decision = (body && body.decision) || "";
        if (decision === "PROCEED_CURRENT") {
          const grade = st.reviewGrade || "";
          if ((grade === "B" || grade === "NOT_READY") && !(body && body.acceptQualityRisk))
            err(400, `等级为 ${grade}：必须勾选「我已知悉质量风险」后才能接受当前稿`);
          st.status = "LOCKED";
          st.confirmedAt = iso();
          r.run.status = "COMPLETED";
          r.run.currentStage = "";
          return scriptDetail(r);
        }
        if (decision === "REFINE_OR_REWRITE") {
          st.status = "LOCKED";
          st.confirmedAt = iso();
          POLISH_DEFS.forEach(([k, t]) => r.stages.push({ id: id("stg"), key: k, title: t, status: "NOT_STARTED", version: 0, artifactId: "", readyAt: 0, reviewGrade: "", draft: null, confirmedAt: "" }));
          startStage(r, findStage(r, "polish_draft"));
          return scriptDetail(r);
        }
        err(400, "未知决策类型：" + decision);
      }
    }
    err(404, "仿真后端未知端点：" + method + " " + path);
  }

  function assetRoutes(pid, method, s, body) {
    if (method === "POST" && !s[3]) {
      const L = db.locks[pid];
      if (!L) err(409, "请先提交立项锁");
      const sr = db.runs[body && body.scriptRunId];
      if (!sr || sr.kind !== "script" || sr.run.status !== "COMPLETED")
        err(409, "剧本任务尚未完成，无法创建资产任务");
      const r = { projectId: pid, kind: "asset", locks: L, stages: mkStages(ASSET_DEFS), artifacts: [], images: [], videos: [], previs: null };
      r.run = { id: id("arun"), status: "ACTIVE", branch: sr.run.branch, assetMode: (body && body.assetMode) || L.assetMode || "M1", currentStage: "" };
      db.runs[r.run.id] = r;
      startStage(r, r.stages[0]);
      return assetDetail(r);
    }
    const r = findRun(pid, "asset", s[3]);
    if (method === "GET" && !s[4]) return assetDetail(r);
    if (method === "POST" && s[4] === "stages") {
      const st = findStage(r, s[5]);
      if (s[6] === "run") {
        if (st.status !== "NOT_STARTED" && st.status !== "BLOCKED" && st.status !== "STALE")
          err(409, `「${st.title}」当前状态为 ${st.status}，不可执行`);
        startStage(r, st);
        return assetDetail(r);
      }
    }
    if (method === "POST" && s[4] === "confirm-manifest") {
      const m = findStage(r, "manifest");
      if (m.status !== "AWAITING_CONFIRMATION" || !m.artifactId)
        err(409, "资产清单尚未生成完成，不可确认");
      m.status = "LOCKED";
      m.confirmedAt = iso();
      startStage(r, findStage(r, "compile"));
      return assetDetail(r);
    }
    if (method === "POST" && s[4] === "confirm") {
      const c = findStage(r, "compile");
      if (c.status !== "AWAITING_CONFIRMATION" || !c.artifactId)
        err(409, "资产交付尚未编译完成，不可确认");
      c.status = "LOCKED";
      c.confirmedAt = iso();
      r.run.status = "COMPLETED";
      r.run.currentStage = "";
      return assetDetail(r);
    }
    err(404, "仿真后端未知端点：" + method + " " + path);
  }

  function prodRoutes(pid, method, s, body) {
    if (method === "POST" && !s[3]) {
      const L = db.locks[pid];
      if (!L) err(409, "请先提交立项锁");
      const ar = db.runs[body && body.assetRunId];
      if (!ar || ar.kind !== "asset" || ar.run.status !== "COMPLETED")
        err(409, "资产任务尚未完成，无法创建生产链");
      const r = { projectId: pid, kind: "production", locks: L, stages: mkStages(PROD_DEFS), artifacts: [], images: [], videos: [], previs: null };
      r.run = { id: id("prun"), status: "ACTIVE", branch: ar.run.branch, assetMode: ar.run.assetMode, currentStage: "" };
      db.runs[r.run.id] = r;
      startStage(r, r.stages[0]);
      return prodDetail(r);
    }
    const r = findRun(pid, "production", s[3]);
    if (method === "GET" && !s[4]) return prodDetail(r);
    const sb = findStage(r, "storyboard");
    const feed = findStage(r, "feed");
    if (method === "POST" && s[4] === "storyboard-images") {
      if (sb.status !== "AWAITING_CONFIRMATION" || !sb.artifactId)
        err(409, "P3 分镜尚未生成完成，不能启动故事板图片");
      if (r.images.length) err(409, "故事板图片批次已存在，请等待就绪");
      r.images = [1, 2, 3].map((i) => ({ id: id("img"), code: "段" + i, status: "GENERATING", readyAt: now() + i * SIM_MEDIA_MS }));
      return prodDetail(r);
    }
    if (method === "POST" && s[4] === "previs-storyboard") {
      if (feed.status !== "AWAITING_CONFIRMATION" || !feed.artifactId)
        err(409, "P4 投喂稿尚未生成完成，不能启动预演");
      if (r.previs && (r.previs.status === "READY" || r.previs.status === "RUNNING"))
        err(409, "预演故事板已存在");
      r.previs = { status: "RUNNING", readyAt: now() + SIM_MEDIA_MS * 2 };
      return prodDetail(r);
    }
    if (method === "POST" && s[4] === "videos") {
      if (!r.previs || r.previs.status !== "READY")
        err(409, "P4.5 预演故事板尚未就绪，不能启动视频批次");
      if (r.videos.length) err(409, "视频批次已存在，请等待就绪");
      r.videos = [1, 2].map((i) => ({ id: id("vid"), code: "段" + i, status: "GENERATING", readyAt: now() + i * SIM_MEDIA_MS * 1.5 }));
      return prodDetail(r);
    }
    if (method === "POST" && s[4] === "stages") {
      const st = findStage(r, s[5]);
      if (s[6] === "run") {
        if (st.status !== "NOT_STARTED" && st.status !== "BLOCKED" && st.status !== "STALE")
          err(409, `「${st.title}」当前状态为 ${st.status}，不可执行`);
        startStage(r, st);
        return prodDetail(r);
      }
      if (s[6] === "confirm") {
        if (st.status !== "AWAITING_CONFIRMATION" || !st.artifactId)
          err(409, `「${st.title}」尚未生成完成，不可确认`);
        if (st.key === "storyboard") {
          const ready = r.images.filter((i) => i.status === "READY").length;
          if (!r.images.length || ready < r.images.length)
            err(409, `故事板图片尚未全部就绪（${ready}/${r.images.length}），不能锁定 P3`);
        }
        if (st.key === "feed") {
          const vReady = r.videos.filter((v) => v.status === "READY").length;
          if (!r.previs || r.previs.status !== "READY" || !r.videos.length || vReady < r.videos.length)
            err(409, `预演/视频尚未全部就绪（视频 ${vReady}/${r.videos.length}），不能锁定 P4`);
        }
        lockAfter(r, st);
        if (st.key === "qc" && r.run.status === "COMPLETED") buildDeliverables(r);
        return prodDetail(r);
      }
    }
    err(404, "仿真后端未知端点：" + method + " " + path);
  }

  return { handle };
})();

/* ============================================================
 * 交互式阶段推进引擎（真实模式）
 * ============================================================ */
const ENGINE = {
  running: false,
  timer: null,
  projectId: "",
  scriptRunId: "",
  assetRunId: "",
  productionRunId: "",
  locks: null,
  scriptDetail: null,
  assetDetail: null,
  productionDetail: null,
  pollTick: 0,
};

const POLL_MS = 3000;
const SCRIPT_GATE_CONFIRM = new Set(["polish_confirmation"]);
const PROD_GATE_CONFIRM = new Set(["storyboard", "feed", "qc"]);
const STAGE_STATUS_META = {
  NOT_STARTED: { label: "待执行", cls: "" },
  IN_PROGRESS: { label: "生成中", cls: "run" },
  AWAITING_CONFIRMATION: { label: "待确认", cls: "wait" },
  AWAITING_USER_DECISION: { label: "待你决策", cls: "wait" },
  LOCKED: { label: "已锁定", cls: "ok" },
  BLOCKED: { label: "已阻塞", cls: "bad" },
  STALE: { label: "需重跑", cls: "wait" },
};

function isAuto() { return $("#autoRun").checked; }

function saveSession() {
  const s = {
    projectId: ENGINE.projectId, scriptRunId: ENGINE.scriptRunId,
    assetRunId: ENGINE.assetRunId, productionRunId: ENGINE.productionRunId,
    savedAt: new Date().toISOString(),
    lockSummary: ENGINE.locks ? lockSummary(ENGINE.locks) : "",
  };
  localStorage.setItem("vd_session", JSON.stringify(s));
}
function loadSession() {
  try { return JSON.parse(localStorage.getItem("vd_session") || "null"); } catch { return null; }
}
function clearSession() { localStorage.removeItem("vd_session"); }

/* ---------- 主入口 ---------- */
function onRun() {
  const locks = collectLocks();
  if (!locks) return;
  if (CONFIG.mode !== "mock") { startRealRun(locks); return; }

  $("#stepRun").classList.remove("hidden");
  $("#stepRun").scrollIntoView({ behavior: "smooth" });
  clearLog();
  runMock(locks);
}

async function startRealRun(locks) {
  stopEngine(true);
  ENGINE.projectId = "portal-" + Date.now().toString(36);
  ENGINE.scriptRunId = ENGINE.assetRunId = ENGINE.productionRunId = "";
  ENGINE.locks = locks;
  ENGINE.scriptDetail = ENGINE.assetDetail = ENGINE.productionDetail = null;
  ENGINE._deliverablesFetched = false;
  Vault.clear(ENGINE.projectId);
  clearSession();

  $("#stepRun").classList.remove("hidden");
  $("#resumeBar").classList.add("hidden");
  // 文件中心随运行开始即显示，产物落库后实时刷新
  _lastFileCount = -1;
  $("#stepFiles").classList.remove("hidden");
  renderFiles();
  $("#phaseAsset").classList.add("hidden");
  $("#phaseProduction").classList.add("hidden");
  $("#phaseScriptStatus").textContent = "";
  clearPhaseButton("#phaseScript");
  clearPhaseButton("#phaseAsset");
  $("#stepRun").scrollIntoView({ behavior: "smooth" });
  clearLog();
  setStage("lock", "active");

  try {
    await api(`/projects/${ENGINE.projectId}/lock`, "PUT", lockPayloadOf(locks));
    logLine("▸ 立项锁已锁定：" + lockSummary(locks));
    setStage("lock", "done");

    setStage("script", "active");
    $("#phaseScript").classList.remove("hidden");
    const input = { mode: "STEPWISE" };
    if (locks.trendResolution) input.trendResolution = locks.trendResolution;
    const created = await api(`/projects/${ENGINE.projectId}/script-runs`, "POST", input);
    ENGINE.scriptRunId = created.run.id;
    ENGINE.scriptDetail = created;
    saveSession();
    logLine(`▸ P1 剧本任务已创建（${created.run.branch === "adaptation" ? "改编" : created.run.branch === "benchmark" ? "对标" : "原创"}支线），开始来源登记…`);

    // 登记：把梗概作为来源写入，再交给轮询去确认来源阶段
    await api(`/projects/${ENGINE.projectId}/script-runs/${ENGINE.scriptRunId}/source`, "POST", {
      title: (locks.brief || "创意梗概").slice(0, 24).split("\n")[0] || "创意梗概",
      authorizationScope: locks.source === "adaptation" ? "仅本项目改编" : "仅本项目使用",
      rangeLabel: locks.source === "adaptation" ? "第 1 章" : "完整梗概",
      content: locks.brief,
      metadata: { sourceType: locks.source === "adaptation" ? "novel" : "brief" },
    });
    logLine("▸ 来源已登记（梗概入库）");
    startEngine();
  } catch (e) {
    failAt("script", e, "剧本任务创建");
  }
}

function lockPayloadOf(l) {
  return {
    track: l.track, source: l.source, videoModel: l.videoModel,
    aspectRatio: l.aspectRatio, deliveryMode: l.deliveryMode, seamMode: l.seamMode,
    targetDurationSeconds: l.targetDurationSeconds,
    styleBase: l.styleBase, styleCardId: l.styleCardId,
    styleCustomDescription: l.styleCustomDescription,
    assetMode: l.assetMode, confirmedLocks: l.confirmedLocks,
  };
}

/* ---------- 轮询引擎 ---------- */
function startEngine() {
  if (ENGINE.running) return;
  ENGINE.running = true;
  $("#stopEngine").classList.remove("hidden");
  ENGINE.timer = setInterval(engineTick, POLL_MS);
  engineTick();
}
function stopEngine(silent) {
  ENGINE.running = false;
  if (ENGINE.timer) { clearInterval(ENGINE.timer); ENGINE.timer = null; }
  $("#stopEngine").classList.add("hidden");
  if (!silent) logLine("■ 已停止轮询。阶段产物已保存，可稍后点「继续上次运行」恢复。", false);
}

async function engineTick() {
  ENGINE.pollTick++;
  try {
    if (ENGINE.scriptRunId) await syncScript();
    if (ENGINE.assetRunId) await syncAsset();
    if (ENGINE.productionRunId) await syncProduction();
    syncFilesView(); // 产物入库后文件中心实时刷新
  } catch (e) {
    // 轮询中的网络抖动不打断流程，仅在日志提示
    if (ENGINE.pollTick % 5 === 1) logLine("… 轮询暂不可达（" + (e.message || "网络异常") + "），继续等待", true);
  }
}

/* ---------- P1 剧本 ---------- */
async function syncScript() {
  const d = await api(`/projects/${ENGINE.projectId}/script-runs/${ENGINE.scriptRunId}`, "GET");
  ENGINE.scriptDetail = d;
  vaultFromScript(d);
  renderScriptStages(d);

  if (d.run.status === "COMPLETED") {
    setStage("script", "done");
    phaseStatus("#phaseScriptStatus", "已完成");
    onScriptCompleted(d);
    return;
  }
  const cur = (d.stages || []).find((s) => s.status !== "LOCKED");
  if (!cur) return;
  if (cur.status === "IN_PROGRESS") {
    phaseStatus("#phaseScriptStatus", `「${cur.title}」生成中…`);
    return;
  }
  if (cur.status === "NOT_STARTED" || cur.status === "STALE") {
    phaseStatus("#phaseScriptStatus", `启动「${cur.title}」…`);
    await runScriptStage(cur.key);
    return;
  }
  if (cur.status === "AWAITING_CONFIRMATION") {
    if (SCRIPT_GATE_CONFIRM.has(cur.key)) {
      phaseStatus("#phaseScriptStatus", `「${cur.title}」等待你确认`);
      if (isAuto()) await confirmScriptStage(cur.key);
      return;
    }
    phaseStatus("#phaseScriptStatus", `自动确认「${cur.title}」…`);
    await confirmScriptStage(cur.key);
    return;
  }
  if (cur.status === "AWAITING_USER_DECISION") {
    phaseStatus("#phaseScriptStatus", `圆桌审核等待你决策（等级 ${cur.reviewGrade || "?"}）`);
    if (isAuto()) decideRoundtable("PROCEED_CURRENT", true, "");
    return; // 圆桌永远展示按钮，等用户点
  }
  if (cur.status === "BLOCKED") {
    phaseStatus("#phaseScriptStatus", `「${cur.title}」已阻塞`);
  }
}

async function runScriptStage(key) {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/script-runs/${ENGINE.scriptRunId}/stages/${key}/run`, "POST", { instruction: "" });
    ENGINE.scriptDetail = d;
    logLine(`▸ 已启动阶段「${stageTitle(d, key)}」`);
  } catch (e) {
    if (String(e.message || "").includes("尚未装配")) {
      failAt("script", e, "剧本阶段执行");
      stopEngine(true);
      return;
    }
    logLine("✗ 启动阶段失败：" + (e.message || e), true);
  }
}
async function confirmScriptStage(key) {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/script-runs/${ENGINE.scriptRunId}/stages/${key}/confirm`, "POST", null);
    ENGINE.scriptDetail = d;
    logLine("▸ 已确认锁定「" + stageTitle(d, key) + "」");
  } catch (e) {
    logLine("✗ 确认失败：" + (e.message || e), true);
  }
}
async function decideRoundtable(decision, acceptRisk, note) {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/script-runs/${ENGINE.scriptRunId}/stages/roundtable_review/decide`, "POST", {
      decision, acceptQualityRisk: !!acceptRisk, note: note || "",
    });
    ENGINE.scriptDetail = d;
    logLine(decision === "PROCEED_CURRENT"
      ? "▸ 圆桌决策：接受当前稿" + (acceptRisk ? "（含质量风险确认）" : "")
      : "▸ 圆桌决策：精修重写（追加润色阶段）");
  } catch (e) {
    logLine("✗ 决策失败：" + (e.message || e), true);
  }
}

function onScriptCompleted(d) {
  if (ENGINE.assetRunId || ENGINE._assetStarting) return;
  if (isAuto()) { createAssetRun(); return; }
  phaseStatus("#phaseScriptStatus", "已完成 · 可进入 P2");
  ensurePhaseButton("#phaseScript", "创建 P2 数字资产任务 →", () => createAssetRun());
}

async function createAssetRun() {
  ENGINE._assetStarting = true;
  clearPhaseButton("#phaseScript");
  try {
    const created = await api(`/projects/${ENGINE.projectId}/asset-runs`, "POST", {
      scriptRunId: ENGINE.scriptRunId, assetMode: ENGINE.locks.assetMode,
    });
    ENGINE.assetRunId = created.run.id;
    ENGINE.assetDetail = created;
    ENGINE._assetStarting = false;
    saveSession();
    setStage("asset", "active");
    $("#phaseAsset").classList.remove("hidden");
    logLine("▸ P2 数字资产任务已创建（素材形态 " + created.run.assetMode + "）");
  } catch (e) {
    ENGINE._assetStarting = false;
    failAt("asset", e, "P2 资产任务创建");
  }
}

/* ---------- P2 资产 ---------- */
async function syncAsset() {
  const d = await api(`/projects/${ENGINE.projectId}/asset-runs/${ENGINE.assetRunId}`, "GET");
  ENGINE.assetDetail = d;
  vaultFromAsset(d);
  renderAssetStages(d);

  if (d.run.status === "COMPLETED") {
    setStage("asset", "done");
    phaseStatus("#phaseAssetStatus", "已完成");
    if (!ENGINE.productionRunId && !ENGINE._prodStarting) {
      if (isAuto()) createProductionRun();
      else ensurePhaseButton("#phaseAsset", "创建 P3-P5 生产链 →", () => createProductionRun());
    }
    return;
  }
  const cur = (d.stages || []).find((s) => s.status !== "LOCKED");
  if (!cur) return;
  if (cur.status === "IN_PROGRESS") { phaseStatus("#phaseAssetStatus", `「${cur.title}」生成中…`); return; }
  if (cur.status === "NOT_STARTED" || cur.status === "STALE") {
    await runAssetStage(cur.key);
    return;
  }
  if (cur.status === "AWAITING_CONFIRMATION") {
    phaseStatus("#phaseAssetStatus", `「${cur.title}」等待你确认`);
    if (isAuto()) {
      if (cur.key === "manifest") await confirmAssetManifest();
      else await confirmAssetRun();
    }
    return;
  }
  if (cur.status === "BLOCKED") phaseStatus("#phaseAssetStatus", `「${cur.title}」已阻塞`);
}
async function runAssetStage(key) {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/asset-runs/${ENGINE.assetRunId}/stages/${key}/run`, "POST", { instruction: "" });
    ENGINE.assetDetail = d;
    logLine(`▸ 已启动资产阶段「${stageTitle(d, key)}」`);
  } catch (e) {
    logLine("✗ 资产阶段启动失败：" + (e.message || e), true);
  }
}
async function confirmAssetManifest() {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/asset-runs/${ENGINE.assetRunId}/confirm-manifest`, "POST", {});
    ENGINE.assetDetail = d;
    logLine("▸ 资产清单已确认，进入资产交付编译");
  } catch (e) { logLine("✗ 清单确认失败：" + (e.message || e), true); }
}
async function confirmAssetRun() {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/asset-runs/${ENGINE.assetRunId}/confirm`, "POST", { note: "" });
    ENGINE.assetDetail = d;
    logLine("▸ P2 资产交付已确认锁定");
  } catch (e) { logLine("✗ 资产确认失败：" + (e.message || e), true); }
}

/* ---------- P3-P5 生产 ---------- */
async function createProductionRun() {
  ENGINE._prodStarting = true;
  clearPhaseButton("#phaseAsset");
  try {
    const created = await api(`/projects/${ENGINE.projectId}/production-runs`, "POST", { assetRunId: ENGINE.assetRunId });
    ENGINE.productionRunId = created.run.id;
    ENGINE.productionDetail = created;
    ENGINE._prodStarting = false;
    saveSession();
    setStage("story", "active");
    $("#phaseProduction").classList.remove("hidden");
    logLine("▸ P3-P5 生产链已创建（P0B 决策锚点开始）");
  } catch (e) {
    ENGINE._prodStarting = false;
    failAt("story", e, "P3-P5 生产链创建");
  }
}

async function syncProduction() {
  const d = await api(`/projects/${ENGINE.projectId}/production-runs/${ENGINE.productionRunId}`, "GET");
  ENGINE.productionDetail = d;
  vaultFromProduction(d);
  renderProductionStages(d);

  if (d.run.status === "COMPLETED") {
    ["story", "feed", "deliver"].forEach((s) => setStage(s, "done"));
    phaseStatus("#phaseProductionStatus", "已完成 · 交付物已生成");
    await fetchDeliverables();
    return;
  }
  const cur = (d.stages || []).find((s) => s.status !== "LOCKED");
  if (!cur) return;
  const mapKey = { decision_anchor: "story", storyboard: "story", feed: "feed", qc: "deliver" };
  setStage(mapKey[cur.key] || "story", "active");

  if (cur.status === "IN_PROGRESS") { phaseStatus("#phaseProductionStatus", `「${cur.title}」生成中…`); return; }
  if (cur.status === "NOT_STARTED" || cur.status === "STALE") {
    await runProductionStage(cur.key);
    return;
  }
  if (cur.status === "AWAITING_CONFIRMATION") {
    phaseStatus("#phaseProductionStatus", `「${cur.title}」等待你确认`);
    if (!isAuto()) return; // 展示按钮，等用户
    if (cur.key === "decision_anchor") await confirmProductionStage(cur.key);
    else if (cur.key === "storyboard") await autoStoryboardFlow(d, cur);
    else if (cur.key === "feed") await autoFeedFlow(d, cur);
    else if (cur.key === "qc") await confirmProductionStage(cur.key);
    return;
  }
  if (cur.status === "BLOCKED") phaseStatus("#phaseProductionStatus", `「${cur.title}」已阻塞`);
}

async function autoStoryboardFlow(d, stage) {
  const images = d.images || [];
  const active = images.filter((i) => i.status !== "SUPERSEDED");
  const ready = active.filter((i) => i.status === "READY").length;
  if (!active.length) {
    await startStoryboardImages();
    return;
  }
  if (ready < active.length) return; // 图片生成中，按钮区展示进度
  await confirmProductionStage(stage.key);
}
async function autoFeedFlow(d, stage) {
  const previs = d.previs;
  const videos = d.videos || [];
  if (!previs || (previs.status !== "READY" && previs.status !== "RUNNING")) {
    await startPrevis();
    return;
  }
  if (previs.status !== "READY") return;
  if (!videos.length) { await startVideos(); return; }
  const ready = videos.filter((v) => v.status === "READY").length;
  if (ready < videos.length) return;
  await confirmProductionStage(stage.key);
}

async function runProductionStage(key) {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/production-runs/${ENGINE.productionRunId}/stages/${key}/run`, "POST", { instruction: "" });
    ENGINE.productionDetail = d;
    logLine(`▸ 已启动生产阶段「${stageTitle(d, key)}」`);
  } catch (e) { logLine("✗ 生产阶段启动失败：" + (e.message || e), true); }
}
async function confirmProductionStage(key) {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/production-runs/${ENGINE.productionRunId}/stages/${key}/confirm`, "POST", { note: "" });
    ENGINE.productionDetail = d;
    const t = stageTitle(d, key);
    logLine("▸ 已确认锁定「" + t + "」");
    if (key === "qc") { await fetchDeliverables(); }
  } catch (e) { logLine("✗ 锁定失败：" + (e.message || e), true); }
}
async function startStoryboardImages() {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/production-runs/${ENGINE.productionRunId}/storyboard-images`, "POST", null);
    ENGINE.productionDetail = d;
    logLine("▸ 故事板图片批次已启动（" + (d.images || []).length + " 张排队）");
  } catch (e) { logLine("✗ 图片批次启动失败：" + (e.message || e), true); }
}
async function startPrevis() {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/production-runs/${ENGINE.productionRunId}/previs-storyboard`, "POST", null);
    ENGINE.productionDetail = d;
    logLine("▸ P4.5 预演故事板已启动");
  } catch (e) { logLine("✗ 预演启动失败：" + (e.message || e), true); }
}
async function startVideos() {
  try {
    const d = await api(`/projects/${ENGINE.projectId}/production-runs/${ENGINE.productionRunId}/videos`, "POST", null);
    ENGINE.productionDetail = d;
    logLine("▸ P4 视频批次已启动（" + (d.videos || []).length + " 段排队）");
  } catch (e) { logLine("✗ 视频批次启动失败：" + (e.message || e), true); }
}
async function fetchDeliverables() {
  if (ENGINE._deliverablesFetched) return;
  ENGINE._deliverablesFetched = true;
  try {
    const res = await api(`/projects/${ENGINE.projectId}/deliverables`, "GET");
    const items = res.items || [];
    if (items.length) {
      Vault.upsert(ENGINE.projectId, items.map(deliverableToFile));
      logLine("▸ 最终交付物已入库：" + items.map((i) => i.fileName).join("、"));
      renderFiles();
      $("#stepFiles").classList.remove("hidden");
    }
  } catch (e) {
    logLine("… 交付物暂不可取（" + (e.message || "") + "）", true);
  }
}

/* ---------- 工件入库 ---------- */
function vaultFromScript(d) {
  const t = stageTitleMap(d.stages);
  Vault.upsert(ENGINE.projectId, (d.artifacts || []).map((a) => artifactToFile("P1 剧本", t[a.stageId] || a.kind, a)));
}
function vaultFromAsset(d) {
  const t = stageTitleMap(d.stages);
  Vault.upsert(ENGINE.projectId, (d.artifacts || []).map((a) => artifactToFile("P2 数字资产", t[a.stageId] || a.kind, a)));
}
function vaultFromProduction(d) {
  const idToKey = {};
  (d.stages || []).forEach((s) => (idToKey[s.id] = s.key));
  const phaseOf = { decision_anchor: "P0B 决策", storyboard: "P3 分镜与故事板", feed: "P4 投喂", qc: "P5 最终交付" };
  Vault.upsert(ENGINE.projectId, (d.artifacts || []).map((a) => {
    const key = idToKey[a.stageId] || "";
    return artifactToFile(phaseOf[key] || "生产阶段", key || a.kind, a);
  }));
}
function stageTitleMap(stages) {
  const m = {};
  (stages || []).forEach((s) => (m[s.id] = s.title));
  return m;
}
function stageTitle(detail, key) {
  const s = (detail && detail.stages || []).find((x) => x.key === key);
  return s ? s.title : key;
}

/* ============================================================
 * 阶段行渲染
 * ============================================================ */
function stageRow(phase, st, extraActions) {
  const meta = STAGE_STATUS_META[st.status] || { label: st.status, cls: "" };
  const dotCls = meta.cls === "ok" ? "done" : meta.cls === "bad" ? "bad" : meta.cls === "wait" ? "wait" : meta.cls === "run" ? "active" : "";
  let actions = extraActions || "";
  if (!actions) {
    // 按 phase 路由执行/重试动作到正确的 run 端点
    const opMap = { script: "runScript", asset: "runAsset", production: "runProd" };
    const op = opMap[phase] || "runScript";
    if (st.status === "NOT_STARTED" || st.status === "STALE") actions = `<button class="mini-btn primary" data-act="${op}:${st.key}">执行</button>`;
    if (st.status === "BLOCKED") actions = `<button class="mini-btn" data-act="${op}:${st.key}">重试</button>`;
  }
  const sub = [];
  if (st.version > 0) sub.push("v" + st.version);
  if (st.error) sub.push(st.error.slice(0, 120));
  if (st.confirmedAt) sub.push("确认于 " + new Date(st.confirmedAt).toLocaleTimeString("zh-CN"));
  return `
    <div class="stage-row" data-key="${escAttr(st.key)}">
      <span class="st-dot ${dotCls}"></span>
      <div class="st-main">
        <div class="st-name">${escHtml(st.title || st.key)}</div>
        ${sub.length ? `<div class="st-sub">${escHtml(sub.join(" · "))}</div>` : ""}
        ${gateExtras(phase, st)}
      </div>
      <span class="st-chip ${meta.cls}">${meta.label}</span>
      <div class="st-actions">${actions}</div>
    </div>`;
}

// 门控扩展区：圆桌决策 / 润色回签 / 资产清单与交付 / P3 图片 / P4 预演+视频
function gateExtras(phase, st) {
  if (phase === "script" && st.status === "AWAITING_CONFIRMATION") {
    return `
      <div class="gate-box">
        <div class="gate-title">⏸ 「${escHtml(st.title || st.key)}」已生成，等待你确认锁定。</div>
        <div class="gate-acts">
          <button class="mini-btn primary" data-act="confirmScript:${st.key}">✓ 确认并锁定</button>
        </div>
      </div>`;
  }
  if (phase === "script" && st.key === "roundtable_review" && st.status === "AWAITING_USER_DECISION") {
    const grade = st.reviewGrade || "";
    const risky = grade === "B" || grade === "NOT_READY";
    const draftBtn = st.draft ? `<button class="mini-btn" data-act="viewDraft:${st.key}">查看审核稿</button>` : "";
    return `
      <div class="gate-box">
        <div class="gate-title">⏸ 五席圆桌审核完成，等级 <b>${escHtml(grade || "?")}</b> —— 请选择：</div>
        <div class="gate-acts">
          <button class="mini-btn primary" data-act="decide:PROCEED_CURRENT">接受当前稿</button>
          <button class="mini-btn" data-act="decide:REFINE_OR_REWRITE">精修重写</button>
          ${draftBtn}
        </div>
        ${risky ? `<label class="risk-line"><input type="checkbox" id="riskAccept" /> 等级为 ${escHtml(grade)}，接受当前稿须勾选「我已知悉质量风险」</label>` : ""}
      </div>`;
  }
  if (phase === "asset" && st.status === "AWAITING_CONFIRMATION") {
    if (st.key === "manifest") {
      return `<div class="gate-box"><div class="gate-acts"><button class="mini-btn primary" data-act="assetManifest">确认资产清单，进入编译</button></div></div>`;
    }
    if (st.key === "compile") {
      return `<div class="gate-box"><div class="gate-acts"><button class="mini-btn primary" data-act="assetConfirm">确认 P2 资产交付</button></div></div>`;
    }
  }
  if (phase === "production" && st.status === "AWAITING_CONFIRMATION") {
    const d = ENGINE.productionDetail || {};
    if (st.key === "decision_anchor") {
      return `<div class="gate-box"><div class="gate-acts"><button class="mini-btn primary" data-act="prodConfirm:decision_anchor">锁定 P0B 决策锚点</button></div></div>`;
    }
    if (st.key === "storyboard") {
      const images = (d.images || []).filter((i) => i.status !== "SUPERSEDED");
      const ready = images.filter((i) => i.status === "READY").length;
      const failed = images.filter((i) => i.status === "FAILED").length;
      let progress = "";
      if (images.length) {
        progress = `<div class="media-progress">故事板图片：${ready}/${images.length} 就绪${failed ? ` · <span class="bad-text">${failed} 失败</span>` : ""}</div>`;
      }
      return `
        <div class="gate-box">
          <div class="gate-title">P3 分镜已生成。锁定前需先取得全部故事板图片。</div>
          ${progress}
          <div class="gate-acts">
            ${images.length ? "" : `<button class="mini-btn primary" data-act="prodImages">① 生成故事板图片</button>`}
            ${ready >= images.length && images.length ? `<button class="mini-btn primary" data-act="prodConfirm:storyboard">② 锁定 P3</button>` : ""}
          </div>
        </div>`;
    }
    if (st.key === "feed") {
      const previs = d.previs;
      const videos = d.videos || [];
      const vReady = videos.filter((v) => v.status === "READY").length;
      const lines = [];
      lines.push(`P4.5 预演故事板：${previs ? (previs.status === "READY" ? "✅ 就绪" : previs.status) : "未生成"}`);
      if (videos.length) lines.push(`P4 视频：${vReady}/${videos.length} 就绪`);
      return `
        <div class="gate-box">
          <div class="gate-title">P4 投喂稿已生成。锁定前需：预演故事板通过 → 全部视频就绪。</div>
          <div class="media-progress">${lines.map(escHtml).join("<br/>")}</div>
          <div class="gate-acts">
            ${!previs || previs.status === "FAILED" ? `<button class="mini-btn primary" data-act="prodPrevis">① 生成预演故事板</button>` : ""}
            ${previs && previs.status === "READY" && !videos.length ? `<button class="mini-btn primary" data-act="prodVideos">② 生成视频批次</button>` : ""}
            ${videos.length && vReady >= videos.length ? `<button class="mini-btn primary" data-act="prodConfirm:feed">③ 锁定 P4</button>` : ""}
          </div>
        </div>`;
    }
    if (st.key === "qc") {
      return `<div class="gate-box"><div class="gate-acts"><button class="mini-btn primary" data-act="prodConfirm:qc">生成最终交付并锁定 P5</button></div></div>`;
    }
  }
  return "";
}

function renderScriptStages(d) {
  $("#scriptStages").innerHTML = (d.stages || [])
    .map((s) => stageRow("script", s))
    .join("");
  bindStageActions("#scriptStages");
}
function renderAssetStages(d) {
  $("#assetStages").innerHTML = (d.stages || [])
    .map((s) => stageRow("asset", s))
    .join("");
  bindStageActions("#assetStages");
}
function renderProductionStages(d) {
  $("#productionStages").innerHTML = (d.stages || [])
    .map((s) => stageRow("production", s))
    .join("");
  bindStageActions("#productionStages");
}

/* ---------- 门控按钮动作绑定 ---------- */
function bindStageActions(sel) {
  const root = $(sel);
  root.querySelectorAll("[data-act]").forEach((btn) => {
    btn.onclick = async () => {
      const act = btn.dataset.act;
      const [op, arg] = act.split(":");
      btn.disabled = true;
      try {
        if (op === "runScript") await runScriptStage(arg);
        else if (op === "runAsset") await runAssetStage(arg);
        else if (op === "runProd") await runProductionStage(arg);
        else if (op === "confirmScript") await confirmScriptStage(arg);
        else if (op === "decide") {
          const risky = $("#riskAccept");
          if (arg === "PROCEED_CURRENT" && risky && !risky.checked) {
            logLine("✗ 等级为 B/NOT_READY：请先勾选「已知悉质量风险」", true);
            btn.disabled = false;
            return;
          }
          await decideRoundtable(arg, arg === "PROCEED_CURRENT" && !!risky?.checked, "");
        }
        else if (op === "viewDraft") {
          const st = (ENGINE.scriptDetail?.stages || []).find((s) => s.key === arg);
          if (st?.draft) Vault.upsert(ENGINE.projectId, [{
            uid: `审核稿:${st.id}:${st.version}`, phase: "P1 剧本", stage: st.title,
            kind: "ROUNDTABLE_REVIEW", version: st.version,
            name: `P1-圆桌审核-v${st.version}.md`, content: st.draft.content || "",
          }]);
          const f = Vault.list(ENGINE.projectId).find((x) => x.uid.startsWith("审核稿:"));
          if (f) openViewer(f.uid);
        }
        else if (op === "assetManifest") await confirmAssetManifest();
        else if (op === "assetConfirm") await confirmAssetRun();
        else if (op === "prodConfirm") await confirmProductionStage(arg);
        else if (op === "prodImages") await startStoryboardImages();
        else if (op === "prodPrevis") await startPrevis();
        else if (op === "prodVideos") await startVideos();
      } finally {
        btn.disabled = false;
        engineTick();
      }
    };
  });
}

function ensurePhaseButton(phaseSel, label, fn) {
  const panel = $(phaseSel);
  if (panel.querySelector(".phase-next")) return;
  const wrap = document.createElement("div");
  wrap.className = "phase-next";
  const btn = document.createElement("button");
  btn.className = "primary-btn";
  btn.textContent = label;
  btn.onclick = fn;
  wrap.appendChild(btn);
  panel.appendChild(wrap);
}
function clearPhaseButton(phaseSel) {
  const el = $(phaseSel).querySelector(".phase-next");
  if (el) el.remove();
}

/* ---------- 管线可视化 / 日志 ---------- */
const STAGES = ["lock", "script", "asset", "story", "feed", "deliver"];
function setStage(name, state) {
  const li = $(`.pipeline li[data-stage="${name}"]`);
  if (!li) return;
  li.classList.remove("active", "done", "error");
  if (state) li.classList.add(state);
}
function logLine(msg, isErr) {
  const d = document.createElement("div");
  if (isErr) d.className = "err";
  d.textContent = new Date().toLocaleTimeString("zh-CN") + " " + msg;
  $("#runLog").appendChild(d);
  $("#runLog").scrollTop = $("#runLog").scrollHeight;
}
function clearLog() {
  $("#runLog").innerHTML = "";
  STAGES.forEach((s) => setStage(s, null));
  hideError();
}
function phaseStatus(sel, text) { $(sel).textContent = text; }

function failAt(stage, e, label) {
  stopEngine(true);
  setStage(stage, "error");
  const info = describeBackendError(e, label);
  logLine("✗ " + info.title + (e.message ? "：" + e.message : ""), true);
  showError(info.title, e.message, info.fix);
}

/* ---------- 错误翻译（保留） ---------- */
function describeBackendError(e, label) {
  if (e.kind === "network") {
    return {
      title: `无法连接后端（${CONFIG.origin}）`,
      fix: "请检查右上角「连接设置」里的后端地址是否正确、后端服务是否已启动；若后端在其它机器，确认网络可达且端口已开放（默认 8790）。也可先用模拟模式验证产品形态。",
    };
  }
  const status = e.status || 0;
  const msg = e.message || "HTTP " + status;
  let fix = "";
  if (status === 401) {
    fix = "访问令牌无效或缺失。真实模式必须带 Bearer 令牌：请在右上角「连接设置」里填写正确的令牌后重试。";
  } else if (status === 403) {
    fix = "当前账号无操作权限，或令牌对应的用户没有该项目权限。请换用有权限的账号/令牌。";
  } else if (status === 404) {
    fix = "请求的资源不存在。若刷新后出现，可能是会话已过期或项目已被删除；请放弃本次运行后重新立项。";
  } else if (status === 400) {
    if (/段缝/.test(msg)) fix = "目标时长需要多段生成：请先选择「D · 段缝衔接方式」再提交立项锁。";
    else if (/模型|绑定|能力匹配/.test(msg)) fix = "所选模型未被后台启用或能力不匹配：请在画布后台确认该模型已启用且支持当前画风基底/视频形态，再回来重选。";
    else if (/确认|锁定|confirmedLocks/.test(msg)) fix = "P0 硬锁必须逐项确认：门户已自动带上逐项确认；若仍报错请检查后端版本。";
    else if (/立项锁|lock|风格|基底|兼容|参数无效/.test(msg)) fix = "立项锁参数未通过后端校验。请确认：① 所有六道锁字段都已选择；② 风格卡与画风基底兼容；③ 所选模型已在后台启用。";
    else fix = "请求参数不合法。请核对表单是否填写完整，或查看后端日志获取具体原因。";
  } else if (status === 409) {
    if (/重新生成 P3/.test(msg)) fix = "P3 工件未通过契约校验：点击 P3 行的「重试」仅重新生成 P3，历史工件与媒体会保留。";
    else fix = "状态冲突：请先完成当前阶段的确认或等待生成结束，再执行下一步。";
  } else if (status >= 500) {
    fix = "后端服务内部错误。请稍后重试；若持续失败，联系后端运维或查看服务日志。";
  } else {
    fix = "请查看后端返回信息，或切换为模拟模式先验证产品形态。";
  }
  return { title: `${label}失败 · 后端返回 ${status}`, fix };
}
function showError(title, msg, fix) {
  $("#errTitle").textContent = title;
  $("#errMsg").textContent = msg || "";
  $("#errFix").textContent = fix || "";
  $("#errBanner").classList.remove("hidden");
  $("#errBanner").scrollIntoView({ behavior: "smooth" });
}
function hideError() { $("#errBanner").classList.add("hidden"); }

/* ============================================================
 * 会话恢复：刷新后继续未完成运行
 * ============================================================ */
async function tryResume() {
  if (CONFIG.mode !== "real") return; // 仿真/模拟模式运行态保存在页面内，刷新即清空
  const s = loadSession();
  if (!s || !s.projectId || !s.scriptRunId) return;
  $("#resumeBar").classList.remove("hidden");
  $("#resumeText").textContent = `检测到未完成的项目 ${s.projectId}（${s.lockSummary || "已立项"}）`;
  $("#resumeGo").onclick = async () => {
    ENGINE.projectId = s.projectId;
    ENGINE.scriptRunId = s.scriptRunId || "";
    ENGINE.assetRunId = s.assetRunId || "";
    ENGINE.productionRunId = s.productionRunId || "";
    ENGINE.locks = null;
    $("#resumeBar").classList.add("hidden");
    $("#stepRun").classList.remove("hidden");
    $("#phaseScript").classList.remove("hidden");
    if (ENGINE.assetRunId) $("#phaseAsset").classList.remove("hidden");
    if (ENGINE.productionRunId) $("#phaseProduction").classList.remove("hidden");
    clearLog();
    logLine("▸ 已恢复会话，正在同步各阶段状态…");
    startEngine();
  };
  $("#resumeDrop").onclick = () => {
    clearSession();
    $("#resumeBar").classList.add("hidden");
  };
}

/* ============================================================
 * 模拟模式
 * ============================================================ */
function runMock(locks) {
  $("#phaseScript").classList.remove("hidden");
  ENGINE.projectId = "mock-" + Date.now().toString(36);
  _lastFileCount = -1;
  $("#stepFiles").classList.remove("hidden");
  renderFiles(); // 空态提示，产物就绪后即时出现
  const steps = [
    ["lock", "立项锁校验通过 ✓ " + lockSummary(locks)],
    ["script", "剧本生成：按" + locks._labels.source + " / " + locks._labels.track + " 产出分集剧本与台词"],
    ["asset", "资产提示词：提取角色 / 场景 / 道具，按 " + locks.assetMode + " 产出"],
    ["story", "分镜图提示词：按 " + locks._labels.model + " / " + locks._labels.aspect + " 编排镜头"],
    ["feed", "投喂稿：按七段式封装，段缝=" + locks._labels.seam],
    ["deliver", "交付汇总：形态=" + locks._labels.delivery],
  ];
  steps.forEach(([stage, msg], i) => {
    setTimeout(() => {
      setStage(stage, "active");
      logLine("▸ " + msg);
      setTimeout(() => setStage(stage, "done"), 320);
      if (i === steps.length - 1) setTimeout(() => showMockDeliver(locks), 250);
    }, i * 420);
  });
}

function lockSummary(l) {
  const labels = l._labels || {};
  return [labels.track || l.track, labels.source || l.source, labels.model || l.videoModel, labels.aspect || l.aspectRatio, styleLabel(l)].join(" · ");
}

function showMockDeliver(l) {
  const d = mockDeliverables(l);
  Vault.upsert(ENGINE.projectId, [
    { uid: "mock:asset", phase: "示例交付", stage: "资产提示词", kind: "ASSET_PROMPT", version: 1, name: "示例-资产提示词.md", content: d.asset },
    { uid: "mock:story", phase: "示例交付", stage: "分镜图提示词", kind: "STORYBOARD", version: 1, name: "示例-分镜图提示词.md", content: d.story },
    { uid: "mock:feed", phase: "示例交付", stage: "投喂稿", kind: "FEED", version: 1, name: "示例-投喂稿.md", content: d.feed },
  ]);
  $("#stepFiles").classList.remove("hidden");
  $("#stepFiles").scrollIntoView({ behavior: "smooth" });
  renderFiles();
}

function mockDeliverables(l) {
  const aspect = l.aspectRatio.includes("竖") ? "9:16" : l.aspectRatio.includes("宽") ? "21:9" : "16:9";
  const baseName = (CATALOG.bases || []).find((b) => b.id === l.styleBase);
  const styleNote = styleNoteOf(l) + (baseName ? ` ｜ 基底：${baseName.name}` : "");

  const asset = `# 资产提示词（${l.assetMode}）

> 模型 ${l.videoModel} · ${aspect} · ${styleNote}

## 角色
- 女主·沈昭：25岁，冷白皮，齐肩黑发，重生后眼神由怯转厉。
  提示词：电影感肖像，冷调侧光，面部特写，高对比，情绪克制，${styleNote}。
- 反派·林婉：温婉表象下的算计，珍珠耳环，旗袍。
  提示词：低调奢华室内光，半脸阴影，危险甜美，胶片颗粒。

## 场景
- 豪宅客厅（夜间）：水晶灯，深红丝绒沙发，落地窗雨夜。
  提示词：${aspect} 构图，霓虹城市背景虚化，戏剧性顶光。
- 公司会议室：冷白日光灯，长桌，玻璃幕墙。
  提示词：极简商务空间，广角，冷色调。

## 道具
- 旧怀表：推动重生回忆的关键物。
  提示词：微距，金属质感，做旧黄铜，暖光。

（示例占位，真实模式由 sd-asset-engine 按素材形态生成图号与接线单）`;

  const story = `# 分镜图提示词（${l.videoModel} / ${aspect}）

> ${styleNote} · 段缝=${l.seamMode}

【镜1】近景·沈昭睁眼（重生瞬间）
画面：雨夜车窗倒影中她猛然睁眼，瞳孔收缩。
提示词：${aspect} 近景，车窗冷蓝反光，湿发贴额，惊惶转清醒，电影感。

【镜2】中景·林婉递酒（伪善）
画面：林婉微笑递杯，指尖红蔻丹。
提示词：暖厅冷人对比，浅景深，危险甜美，慢推。

【镜3】特写·怀表落地（命运转折）
画面：怀表坠地，表盖弹开停在死亡时刻。
提示词：微距慢镜，黄铜反光，尘埃浮动，戏剧光。

【镜4】全景·沈昭立于落地窗前（立誓）
画面：背对镜头，城市霓虹勾勒轮廓。
提示词：${aspect} 剪影，前景虚化，史诗感，蓄势。

（示例占位，真实模式由 sd-storyboard-engine + 打戏引擎按动作档产出）`;

  const feed = `# 投喂稿（七段式 · ${l.videoModel}）

【核心人物】
沈昭（声线：清冷→隐忍）、林婉（声线：甜腻→阴冷）

【镜头1】${aspect} 近景 0-3s
提示词：${story.split("【镜1】")[1].split("提示词：")[1].split("。")[0]}。
台词：（画外）"这一世，我不会再信你。"

【镜头2】${aspect} 中景 3-7s
提示词：暖厅冷人对比，浅景深。
台词：林婉"妹妹，喝了吧，暖暖身子。"

【镜头3】${aspect} 特写 7-11s
提示词：微距慢镜，黄铜反光。
台词：（无声·表盖弹开）

【镜头4】${aspect} 全景 11-15s
提示词：${aspect} 剪影，史诗感。
台词：（画外）"游戏，才刚开始。"

【段缝】${l.seamMode} 衔接至下一段
【交付】形态=${l.deliveryMode} · 模型=${l.videoModel}`;

  return { asset, story, feed };
}

/* ============================================================
 * 表单辅助 / 梗概上传（保留）
 * ============================================================ */
const BRIEF_MAX_BYTES = 2 * 1024 * 1024;

function bindBriefUpload() {
  const fileInput = $("#briefFile");
  const brief = $("#brief");

  $("#briefUploadBtn").addEventListener("click", () => fileInput.click());
  $("#briefClearBtn").addEventListener("click", clearBriefFile);

  fileInput.addEventListener("change", () => {
    if (fileInput.files && fileInput.files[0]) readBriefFile(fileInput.files[0]);
  });

  ["dragenter", "dragover"].forEach((ev) =>
    brief.addEventListener(ev, (e) => { e.preventDefault(); brief.classList.add("dragover"); }));
  ["dragleave", "drop"].forEach((ev) =>
    brief.addEventListener(ev, (e) => { e.preventDefault(); brief.classList.remove("dragover"); }));
  brief.addEventListener("drop", (e) => {
    const f = e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0];
    if (f) readBriefFile(f);
  });
}

function readBriefFile(f) {
  if (f.size > BRIEF_MAX_BYTES) { showBriefInfo("文件过大（上限 2MB）：" + f.name, "err"); return; }
  const reader = new FileReader();
  reader.onload = () => {
    const text = String(reader.result || "");
    if (text.includes("\u0000")) {
      showBriefInfo("不支持二进制文件：" + f.name + "，请上传 .txt / .md 等纯文本", "err");
      return;
    }
    $("#brief").value = text.trim();
    showBriefInfo("已导入：" + f.name + "（" + formatBytes(f.size) + "）", "ok");
    $("#briefClearBtn").classList.remove("hidden");
  };
  reader.onerror = () => showBriefInfo("读取失败：" + f.name, "err");
  reader.readAsText(f, "utf-8");
}
function clearBriefFile() {
  $("#brief").value = "";
  $("#briefFile").value = "";
  showBriefInfo("", "");
  $("#briefClearBtn").classList.add("hidden");
}
function showBriefInfo(msg, cls) {
  const info = $("#briefFileInfo");
  info.textContent = msg;
  info.className = "brief-file-info" + (cls ? " " + cls : "");
}
function formatBytes(n) {
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
  return (n / 1024 / 1024).toFixed(2) + " MB";
}
function copyText(t) {
  navigator.clipboard?.writeText(t).then(() => flash("已复制到剪贴板"), () => flash("复制失败，请手动选择"));
}
function flash(msg) {
  const h = $("#lockHint") || $("#filesHint");
  if (!h) return;
  h.textContent = msg;
  setTimeout(() => (h.textContent = ""), 2200);
}
function escHtml(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}
function escAttr(s) { return escHtml(s); }

/* ============================================================
 * UI 绑定
 * ============================================================ */
function bindUI() {
  loadConfig();
  $("#cfgMode").value = CONFIG.mode;
  $("#cfgOrigin").value = CONFIG.origin;
  $("#cfgToken").value = CONFIG.token;

  $("#openConfig").addEventListener("click", () => $("#configPanel").classList.toggle("hidden"));
  $("#cfgMode").addEventListener("change", (e) => {
    CONFIG.mode = e.target.value; saveConfig();
    if (CONFIG.mode === "real") ensureCatalog();
  });
  $("#cfgOrigin").addEventListener("change", (e) => { CONFIG.origin = e.target.value.trim(); saveConfig(); });
  $("#cfgToken").addEventListener("change", (e) => { CONFIG.token = e.target.value.trim(); saveConfig(); });

  $("#lockSource").addEventListener("change", () => {
    $("#trendResWrap").classList.toggle("hidden", $("#lockSource").value !== "④热点速成");
  });
  $("#lockStyleBase").addEventListener("change", () => populateStyleSelects($("#lockStyleBase").value));
  $("#lockStyleCard").addEventListener("change", refreshStyleHint);
  ensureCatalog();

  bindBriefUpload();

  $("#runBtn").addEventListener("click", onRun);
  $("#stopEngine").addEventListener("click", () => stopEngine(false));
  $("#refreshFiles").addEventListener("click", () => { renderFiles(); flash("已刷新"); });
  $("#fabFiles").addEventListener("click", () => {
    $("#stepFiles").classList.remove("hidden");
    $("#stepFiles").scrollIntoView({ behavior: "smooth" });
  });
  $("#downloadAll").addEventListener("click", downloadAll);

  $("#fvClose").addEventListener("click", closeViewer);
  $("#fvCopy").addEventListener("click", () => fvCurrent && copyText(fvCurrent.content));
  $("#fvDownload").addEventListener("click", () => fvCurrent && downloadBlob(fvCurrent.name, fvCurrent.content, fvCurrent.mime));
  $("#fileViewer").addEventListener("click", (e) => { if (e.target === $("#fileViewer")) closeViewer(); });

  tryResume();
}

bindUI();
