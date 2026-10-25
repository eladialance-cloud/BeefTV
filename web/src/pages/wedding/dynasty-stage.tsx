import { ArrowDown, ArrowUp, Check, Plus, Trash2 } from "lucide-react";

import type { WeddingProductionState } from "@/lib/wedding/types";

const DYNASTY_OPTIONS = [
    ["xia", "夏"], ["xizhou", "西周"], ["warring", "战国"], ["han", "汉"],
    ["jin", "晋"], ["nanbeichao", "南北朝"], ["tang", "唐"], ["song", "宋"],
    ["yuan", "元"], ["ming", "明"], ["qing", "清"], ["minguo", "民国"], ["modern", "现代"],
] as const;

export function DynastyStage({
    production,
    saving,
    feedback,
    onReorder,
    onAdd,
    onRemove,
    onApprove,
}: {
    production: WeddingProductionState;
    saving: boolean;
    feedback: string;
    onReorder: (ids: string[]) => Promise<void>;
    onAdd: (dynasty: { id: string; name: string }) => Promise<void>;
    onRemove: (id: string) => Promise<void>;
    onApprove: () => Promise<void>;
}) {
    const selected = new Set(production.dynasties.map((dynasty) => dynasty.id));
    const available = DYNASTY_OPTIONS.filter(([id]) => !selected.has(id));
    const valid = production.dynasties.length >= 2 && production.dynasties.length <= 5;
    const move = (index: number, direction: -1 | 1) => {
        const next = [...production.dynasties];
        const target = index + direction;
        if (target < 0 || target >= next.length) return;
        [next[index], next[target]] = [next[target]!, next[index]!];
        void onReorder(next.map((dynasty) => dynasty.id));
    };

    return (
        <section className="wedding-dynasty-stage">
            <div className="wedding-stage-panel">
                <header>
                    <div><small>02 · ORDER</small><strong>朝代播放顺序</strong></div>
                    <span>{production.dynasties.length} / 5</span>
                </header>
                <div className="wedding-dynasty-list">
                    {production.dynasties.map((dynasty, index) => (
                        <article key={dynasty.id}>
                            <span className="wedding-dynasty-index">{String(index + 1).padStart(2, "0")}</span>
                            <div>
                                <strong>{dynasty.name}</strong>
                                <small>{dynasty.statuses.video === "approved" ? "视频已确认" : "等待制作"}</small>
                            </div>
                            <div className="wedding-dynasty-actions">
                                <button type="button" aria-label={`上移${dynasty.name}`} disabled={saving || index === 0} onClick={() => move(index, -1)}><ArrowUp /></button>
                                <button type="button" aria-label={`下移${dynasty.name}`} disabled={saving || index === production.dynasties.length - 1} onClick={() => move(index, 1)}><ArrowDown /></button>
                                <button type="button" aria-label={`删除${dynasty.name}`} disabled={saving || production.dynasties.length <= 1} onClick={() => void onRemove(dynasty.id)}><Trash2 /></button>
                            </div>
                        </article>
                    ))}
                </div>
            </div>

            <div className="wedding-stage-panel">
                <header>
                    <div><small>02 · CATALOG</small><strong>新增朝代</strong></div>
                    <span>最多 5 个</span>
                </header>
                <div className="wedding-dynasty-catalog">
                    {available.map(([id, name]) => (
                        <button key={id} type="button" disabled={saving || production.dynasties.length >= 5} onClick={() => void onAdd({ id, name })}>
                            <Plus />
                            {name}
                        </button>
                    ))}
                </div>
                <div className={valid ? "wedding-stage-ready" : "wedding-stage-blockers"}>
                    {valid ? <><Check /> 朝代数量合法，可以确认</> : "请选择 2–5 个朝代"}
                </div>
            </div>

            <footer className="wedding-stage-actions">
                <span>{feedback}</span>
                <button type="button" className="is-primary" disabled={saving || !valid} onClick={() => void onApprove()}>
                    <Check />
                    确认朝代规划
                </button>
            </footer>
        </section>
    );
}
