import {
    ArrowLeft,
    Film,
    Image as ImageIcon,
    Music2,
    Save,
    ScanSearch,
    Sparkles,
} from "lucide-react";
import { useEffect, useState } from "react";

import {
    recommendedWeddingModel,
    weddingModelReady,
    weddingModelSelection,
} from "@/lib/wedding/model-config";
import type {
    WeddingModelSelection,
    WeddingProductionState,
} from "@/lib/wedding/types";
import { ModelPicker } from "@/components/model-picker";
import {
    modelDisplayName,
    type AiConfig,
    type ModelCapability,
} from "@/stores/use-config-store";

const recommendationRows: Array<{
    id: keyof WeddingModelSelection;
    capability: ModelCapability;
    title: string;
    patterns: string[];
    reason: string;
    icon: typeof ImageIcon;
}> = [
    {
        id: "imageModel",
        capability: "image",
        title: "图片生成模型",
        patterns: ["seedream-5", "gpt-image-2.5", "gpt-image-2", "gemini-3.1-flash-image"],
        reason: "用于定妆照和双人场景首帧，优先选择支持多参考图与人物一致性的图片模型。",
        icon: ImageIcon,
    },
    {
        id: "videoModel",
        capability: "video",
        title: "视频生成模型",
        patterns: ["seedance-2.5", "seedance-2.0", "wan3.0-video", "kling-video-v3"],
        reason: "用于朝代视频，优先选择支持参考图生视频、身份稳定和多镜头动作的模型。",
        icon: Film,
    },
    {
        id: "textModel",
        capability: "text",
        title: "视觉质检模型",
        patterns: ["gemini-3.8-flash", "gemini-3.1-pro", "claude-opus-5-5", "gpt-5"],
        reason: "用于场景和视频自动 QC，必须支持图片/视频理解并能稳定返回 JSON。",
        icon: ScanSearch,
    },
    {
        id: "audioModel",
        capability: "audio",
        title: "BGM 音频模型",
        patterns: ["music-generation", "music", "suno", "audio-generation"],
        reason: "用于自动生成器乐 BGM；如果当前目录没有音乐模型，请在包装阶段上传音乐。",
        icon: Music2,
    },
];

export function WeddingModelSelectionStage({
    production,
    config,
    saving,
    feedback,
    onSave,
    onBack,
}: {
    production: WeddingProductionState;
    config: AiConfig;
    saving: boolean;
    feedback: string;
    onSave: (models: WeddingModelSelection) => Promise<void>;
    onBack: () => void;
}) {
    const [draft, setDraft] = useState(() => weddingModelSelection(production, config));
    useEffect(() => {
        setDraft(weddingModelSelection(production, config));
    }, [config, production.models, production.source, production.template]);

    return (
        <section className="wedding-model-stage">
            <header className="wedding-model-header">
                <div>
                    <small>MODEL ROUTING</small>
                    <h1>模型选择</h1>
                    <p>模型选择只应用于当前婚礼项目，不会修改其他画布或账号默认模型。</p>
                </div>
                <button type="button" onClick={onBack}>
                    <ArrowLeft />
                    返回制作台
                </button>
            </header>

            <div className="wedding-model-grid">
                {recommendationRows.map((row) => {
                    const Icon = row.icon;
                    const recommended = recommendedWeddingModel(config, row.capability, row.patterns);
                    return (
                        <section key={row.id} className="wedding-model-card">
                            <header>
                                <div>
                                    <Icon />
                                    <span>
                                        <strong>{row.title}</strong>
                                        <small>{row.reason}</small>
                                    </span>
                                </div>
                                <span className={recommended ? "is-available" : "is-missing"}>
                                    {recommended ? "推荐可用" : "当前目录未匹配"}
                                </span>
                            </header>
                            <ModelPicker
                                config={config}
                                capability={row.capability}
                                value={draft[row.id]}
                                onChange={(model) => setDraft((current) => ({ ...current, [row.id]: model }))}
                                fullWidth
                                placeholder={`选择${row.title}`}
                                showConfiguredModelName
                            />
                            <div className="wedding-model-recommendation">
                                <Sparkles />
                                <span>
                                    <small>推荐</small>
                                    <strong>{recommended || "目录中暂无匹配模型"}</strong>
                                </span>
                                {recommended && recommended !== draft[row.id] ? (
                                    <button type="button" onClick={() => setDraft((current) => ({ ...current, [row.id]: recommended }))}>
                                        使用推荐
                                    </button>
                                ) : null}
                            </div>
                        </section>
                    );
                })}
            </div>

            <section className="wedding-model-table-panel">
                <header>
                    <div>
                        <strong>推荐模型表</strong>
                        <small>推荐项会根据当前账号已上架的模型目录自动匹配。</small>
                    </div>
                </header>
                <div className="wedding-model-table-wrap">
                    <table>
                        <thead>
                            <tr>
                                <th>制作环节</th>
                                <th>首选建议</th>
                                <th>当前上架推荐</th>
                                <th>选择原则</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>定妆照 / 场景首帧</td>
                                <td>Seedream 5.0 / GPT Image 2.5</td>
                                <td>{recommendationLabel(config, "image", ["seedream-5", "gpt-image-2.5", "gpt-image-2"])}</td>
                                <td>多参考图、人脸稳定、中文服饰与场景一致性优先。</td>
                            </tr>
                            <tr>
                                <td>朝代视频</td>
                                <td>Seedance 2.5 / Seedance 2.0</td>
                                <td>{recommendationLabel(config, "video", ["seedance-2.5", "seedance-2.0", "wan3.0-video"])}</td>
                                <td>必须支持参考图生视频；多图参考和人物锁定优先。</td>
                            </tr>
                            <tr>
                                <td>场景 / 视频 QC</td>
                                <td>Gemini 3.8 Flash 等多模态模型</td>
                                <td>{recommendationLabel(config, "text", ["gemini-3.8-flash", "gemini-3.1-pro", "claude-opus-5-5"])}</td>
                                <td>需要同时看图/视频并返回稳定 JSON，普通纯文本模型不够。</td>
                            </tr>
                            <tr>
                                <td>自动 BGM</td>
                                <td>专业音乐生成模型</td>
                                <td>{recommendationLabel(config, "audio", ["music-generation", "music", "suno"])}</td>
                                <td>TTS 配音模型不等同于音乐模型；没有音乐模型时请上传 BGM。</td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </section>

            <footer className="wedding-model-actions">
                <span>{feedback}</span>
                <div>
                    <button type="button" onClick={() => setDraft(weddingModelSelection(production, config))} disabled={saving}>
                        恢复账号默认
                    </button>
                    <button
                        type="button"
                        className="is-primary"
                        disabled={saving || !weddingModelReady(draft)}
                        onClick={() => void onSave(draft)}
                    >
                        <Save />
                        保存模型并返回
                    </button>
                </div>
            </footer>
        </section>
    );
}

function recommendationLabel(config: AiConfig, capability: ModelCapability, patterns: string[]) {
    const model = recommendedWeddingModel(config, capability, patterns);
    return model ? modelDisplayName(config, model) : "未配置";
}
