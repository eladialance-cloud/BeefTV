package hostedapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// manjuModuleDef is one pipeline engine exposed on the admin surface.
type manjuModuleDef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// manjuModuleDefs mirrors the V7.4.26 engine layout (1 controller + 12 sub
// engines). Enabled state is persisted to MANJU_MODULES_FILE when configured;
// without the env the surface is read-only and every module reports enabled.
var manjuModuleDefs = []manjuModuleDef{
	{ID: "short-drama-director", Name: "总控导演", Desc: "六道锁路由与 P0-P5 管线调度"},
	{ID: "sd-decision-engine", Name: "决策引擎", Desc: "赛道分流与方案裁决"},
	{ID: "sd-recon", Name: "考据引擎", Desc: "题材考据与事实校准"},
	{ID: "sd-script", Name: "剧本引擎", Desc: "剧本开发与润色"},
	{ID: "sd-overseas", Name: "海外引擎", Desc: "海外题材本地化"},
	{ID: "sd-jieshuo", Name: "解说引擎", Desc: "解说向文案生产"},
	{ID: "sd-asset", Name: "资产引擎", Desc: "角色/场景资产提示词"},
	{ID: "sd-storyboard", Name: "分镜引擎", Desc: "文戏分镜与故事板"},
	{ID: "sd-combat", Name: "打戏引擎", Desc: "动作/打斗分镜"},
	{ID: "sd-xuanhuan", Name: "玄幻引擎", Desc: "玄幻题材分镜"},
	{ID: "sd-real", Name: "真人实拍引擎", Desc: "真人向分镜与提示词"},
	{ID: "sd-feed", Name: "投喂引擎", Desc: "视频模型投喂稿生成"},
	{ID: "sd-qc", Name: "质检引擎", Desc: "出稿质检与闸门"},
}

// RegisterManjuAdminRoutes mounts the manju admin surface under /manju/admin.
// Every request resolves the current session first and then requires an active
// administrator, matching the other admin consoles.
//
// Stores:
//   - style cards  -> manju.StyleCardStoreImpl (goSource read-only by default;
//     file store when MANJU_STYLE_CARDS_FILE is set)
//   - module state -> MANJU_MODULES_FILE JSON map {"<id>": disabled}
//   - rule files   -> MANJU_SKILL_OVERRIDE directory (read-only when unset)
func RegisterManjuAdminRoutes(r *gin.RouterGroup, resolve ActorResolver, requireAdmin AdminGuard) {
	group := r.Group("/manju/admin")

	actor := func(c *gin.Context) (*model.User, bool) {
		user, err := resolve(c)
		if err != nil {
			writeError(c, err)
			return nil, false
		}
		if err := requireAdmin(user); err != nil {
			writeError(c, err)
			return nil, false
		}
		return user, true
	}

	// ------------------------------------------------------------------
	// Style cards
	// ------------------------------------------------------------------

	group.GET("/style-cards", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		items := manju.ListStyleCards()
		writeOK(c, gin.H{
			"storeConfigured": manju.StyleCardStoreConfigured(),
			"items":           items,
		})
	})

	group.POST("/style-cards", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		var card manju.StyleCard
		if err := bindStrictJSON(c, &card); err != nil {
			writeError(c, kernel.NewAppError(http.StatusBadRequest, "请求体不合法: "+err.Error()))
			return
		}
		if strings.TrimSpace(card.ID) == "" {
			card.Number = nextManjuStyleCardNumber(manju.ListStyleCards(), card.Number)
			card.ID = fmt.Sprintf("sd-style-%03d", card.Number)
		} else if card.Number <= 0 {
			card.Number = parseStyleCardNumber(card.ID)
		}
		if err := validateManjuStyleCard(&card); err != nil {
			writeError(c, kernel.NewAppError(http.StatusBadRequest, err.Error()))
			return
		}
		if err := manju.UpsertStyleCard(card); err != nil {
			writeManjuAdminError(c, err)
			return
		}
		writeOK(c, gin.H{"item": card})
	})

	group.PUT("/style-cards/:id", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		id := c.Param("id")
		if _, exists := manju.GetStyleCard(id); !exists {
			writeError(c, kernel.NewAppError(http.StatusNotFound, "风格卡不存在: "+id))
			return
		}
		var card manju.StyleCard
		if err := bindStrictJSON(c, &card); err != nil {
			writeError(c, kernel.NewAppError(http.StatusBadRequest, "请求体不合法: "+err.Error()))
			return
		}
		card.ID = id
		if card.Number <= 0 {
			card.Number = parseStyleCardNumber(id)
		}
		if err := validateManjuStyleCard(&card); err != nil {
			writeError(c, kernel.NewAppError(http.StatusBadRequest, err.Error()))
			return
		}
		if err := manju.UpsertStyleCard(card); err != nil {
			writeManjuAdminError(c, err)
			return
		}
		writeOK(c, gin.H{"item": card})
	})

	group.DELETE("/style-cards/:id", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		id := c.Param("id")
		if _, exists := manju.GetStyleCard(id); !exists {
			writeError(c, kernel.NewAppError(http.StatusNotFound, "风格卡不存在: "+id))
			return
		}
		if err := manju.DeleteStyleCard(id); err != nil {
			writeManjuAdminError(c, err)
			return
		}
		writeOK(c, gin.H{"deleted": true, "id": id})
	})

	// ------------------------------------------------------------------
	// Modules (enable/disable)
	// ------------------------------------------------------------------

	group.GET("/modules", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		state := readManjuModuleState()
		items := make([]gin.H, 0, len(manjuModuleDefs))
		for _, m := range manjuModuleDefs {
			items = append(items, gin.H{
				"id":      m.ID,
				"name":    m.Name,
				"desc":    m.Desc,
				"enabled": !state[m.ID],
			})
		}
		writeOK(c, gin.H{"storeConfigured": manjuModulesFile() != "", "items": items})
	})

	group.PUT("/modules/:id", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		id := c.Param("id")
		if !knownManjuModule(id) {
			writeError(c, kernel.NewAppError(http.StatusNotFound, "模块不存在: "+id))
			return
		}
		var payload struct {
			Enabled bool `json:"enabled"`
		}
		if err := bindStrictJSON(c, &payload); err != nil {
			writeError(c, kernel.NewAppError(http.StatusBadRequest, "请求体不合法: "+err.Error()))
			return
		}
		state := readManjuModuleState()
		state[id] = !payload.Enabled // persisted map stores "disabled"
		if err := writeManjuModuleState(state); err != nil {
			writeManjuAdminError(c, err)
			return
		}
		writeOK(c, gin.H{"id": id, "enabled": payload.Enabled})
	})

	// ------------------------------------------------------------------
	// Rule files (override layer)
	// ------------------------------------------------------------------

	group.GET("/rule-files", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		files, err := manju.DefaultSkillRuntime().ActiveSkillFiles()
		if err != nil {
			writeError(c, kernel.NewAppError(http.StatusInternalServerError, "读取规则文件清单失败: "+err.Error()))
			return
		}
		items := make([]gin.H, 0, len(files))
		for _, f := range files {
			items = append(items, gin.H{"path": f, "overridden": manju.SkillFileOverridden(f)})
		}
		writeOK(c, gin.H{"overrideDir": manju.SkillOverrideDir(), "items": items})
	})

	group.GET("/rule-files/*path", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		p := strings.TrimPrefix(c.Param("path"), "/")
		content, err := manju.DefaultSkillRuntime().ReadSkillFile(p)
		if err != nil {
			writeError(c, kernel.NewAppError(http.StatusNotFound, "读取规则文件失败: "+err.Error()))
			return
		}
		writeOK(c, gin.H{"path": p, "overridden": manju.SkillFileOverridden(p), "content": string(content)})
	})

	group.PUT("/rule-files/*path", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		p := strings.TrimPrefix(c.Param("path"), "/")
		var payload struct {
			Content string `json:"content"`
		}
		if err := bindStrictJSON(c, &payload); err != nil {
			writeError(c, kernel.NewAppError(http.StatusBadRequest, "请求体不合法: "+err.Error()))
			return
		}
		if err := manju.WriteSkillOverride(p, []byte(payload.Content)); err != nil {
			writeManjuAdminError(c, err)
			return
		}
		writeOK(c, gin.H{"path": p, "overridden": true})
	})
}

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

func manjuModulesFile() string { return os.Getenv("MANJU_MODULES_FILE") }

func knownManjuModule(id string) bool {
	for _, m := range manjuModuleDefs {
		if m.ID == id {
			return true
		}
	}
	return false
}

func readManjuModuleState() map[string]bool {
	state := map[string]bool{}
	if p := manjuModulesFile(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(data, &state)
		}
	}
	return state
}

func writeManjuModuleState(state map[string]bool) error {
	p := manjuModulesFile()
	if strings.TrimSpace(p) == "" {
		return errors.New("MANJU_MODULES_FILE 未配置，模块启停处于只读模式")
	}
	out, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, out, 0o644)
}

func validateManjuStyleCard(card *manju.StyleCard) error {
	card.Name = strings.TrimSpace(card.Name)
	if card.Name == "" {
		return errors.New("风格卡名称不能为空")
	}
	validBases := map[string]bool{}
	for _, b := range manju.StyleCatalog().Bases {
		validBases[b.ID] = true
	}
	if len(card.CompatibleStyleBase) == 0 {
		return errors.New("compatibleStyleBases 不能为空")
	}
	filtered := make([]string, 0, len(card.CompatibleStyleBase))
	for _, b := range card.CompatibleStyleBase {
		b = strings.TrimSpace(b)
		if !validBases[b] {
			return fmt.Errorf("未知画风基底: %s", b)
		}
		filtered = append(filtered, b)
	}
	card.CompatibleStyleBase = filtered
	return nil
}

func nextManjuStyleCardNumber(cards []manju.StyleCard, preferred int) int {
	if preferred > 0 {
		return preferred
	}
	max := 0
	for _, c := range cards {
		if c.Number > max {
			max = c.Number
		}
	}
	return max + 1
}

func parseStyleCardNumber(id string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(id), "sd-style-"))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// writeManjuAdminError maps store-level failures to friendly HTTP errors.
func writeManjuAdminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, manju.ErrStoreReadOnly):
		writeError(c, kernel.NewAppError(http.StatusConflict, err.Error()))
	default:
		writeError(c, kernel.NewAppError(http.StatusBadRequest, err.Error()))
	}
}
