import { AudioLines, Box, Clapperboard, Image, Layers3, Video, WandSparkles } from "lucide-react";

export const beefTVCapabilityItems = [
    { id: "canvas", label: "自由画布", detail: "组织镜头与素材", to: "/canvas?mode=new", icon: Layers3, disabled: false },
    { id: "video", label: "视频生成", detail: "在画布中创建视频节点", to: "/canvas?mode=new&add=video", icon: Video, disabled: false },
    { id: "image", label: "图片生成", detail: "在画布中创建图片节点", to: "/canvas?mode=new&add=image", icon: Image, disabled: false },
    { id: "audio", label: "音频生成", detail: "配音与声音素材", to: "/canvas?mode=new&add=audio", icon: AudioLines, disabled: false },
    { id: "director", label: "3D 导演台", detail: "新建画布并打开 3D 导演台", to: "/canvas?mode=new&start=director", icon: Box, disabled: false },
    { id: "manju", label: "漫剧工作室", detail: "按漫剧老李全流程制作项目", to: "/manju", icon: WandSparkles, disabled: false },
    { id: "projects", label: "项目库", detail: "查看已有项目，继续创作", to: "/project", icon: Clapperboard, disabled: false },
] as const;
