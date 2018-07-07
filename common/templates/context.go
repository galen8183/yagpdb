package templates

import (
	"bytes"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/pkg/errors"
	"strings"
	"text/template"
)

var (
	StandardFuncMap = map[string]interface{}{
		"dict":       Dictionary,
		"json":       tmplJson,
		"in":         in,
		"title":      strings.Title,
		"add":        add,
		"roleAbove":  roleIsAbove,
		"adjective":  common.RandomAdjective,
		"randInt":    randInt,
		"shuffle":    shuffle,
		"seq":        sequence,
		"joinStr":    joinStrings,
		"str":        str,
		"lower":      strings.ToLower,
		"toString":   ToString,
		"toInt":      tmplToInt,
		"toInt64":    ToInt64,
		"formatTime": tmplFormatTime,
		"slice":      slice,
	}

	contextSetupFuncs = []ContextSetupFunc{
		baseContextFuncs,
	}
)

func TODO() {}

type ContextSetupFunc func(ctx *Context)

func RegisterSetupFunc(f ContextSetupFunc) {
	contextSetupFuncs = append(contextSetupFuncs, f)
}

type Context struct {
	GS *dstate.GuildState
	CS *dstate.ChannelState

	MS  *dstate.MemberState
	Msg *discordgo.Message

	BotUser *discordgo.User

	ContextFuncs map[string]interface{}
	Data         map[string]interface{}
	Redis        *redis.Client

	MentionEveryone  bool
	MentionHere      bool
	MentionRoles     []int64
	MentionRoleNames []string

	DelResponse bool
	DelTrigger  bool

	DelTriggerDelay  int
	DelResponseDelay int

	Counters map[string]int
}

func NewContext(gs *dstate.GuildState, cs *dstate.ChannelState, ms *dstate.MemberState) *Context {
	ctx := &Context{
		GS: gs,
		CS: cs,
		MS: ms,

		BotUser: common.BotUser,

		ContextFuncs: make(map[string]interface{}),
		Data:         make(map[string]interface{}),
		Counters:     make(map[string]int),
	}

	ctx.setupContextFuncs()

	return ctx
}

func (c *Context) setupContextFuncs() {
	for _, f := range contextSetupFuncs {
		f(c)
	}
}

func (c *Context) setupBaseData() {

	if c.GS != nil {
		guild := c.GS.LightCopy(false)
		c.Data["Guild"] = guild
		c.Data["Server"] = guild
		c.Data["server"] = guild
	}

	if c.CS != nil {
		channel := c.CS.Copy(false, false)
		c.Data["Channel"] = channel
		c.Data["channel"] = channel
	}

	if c.MS != nil {
		c.Data["Member"] = c.MS.DGoCopy()
		c.Data["User"] = c.MS.DGoUser()
		c.Data["user"] = c.Data["User"]
	}
}

func (c *Context) Execute(redisClient *redis.Client, source string) (string, error) {
	if c.Msg == nil {
		// Construct a fake message
		c.Msg = new(discordgo.Message)
		c.Msg.Author = c.BotUser
		c.Msg.ChannelID = c.GS.ID()
	}

	if c.GS != nil {
		c.GS.RLock()
	}
	c.setupBaseData()
	if c.GS != nil {
		c.GS.RUnlock()
	}

	c.Redis = redisClient

	tmpl := template.New("")
	tmpl.Funcs(StandardFuncMap)
	tmpl.Funcs(c.ContextFuncs)

	parsed, err := tmpl.Parse(source)
	if err != nil {
		return "", errors.WithMessage(err, "Failed parsing template")
	}

	var buf bytes.Buffer
	err = parsed.Execute(&buf, c.Data)

	result := common.EscapeSpecialMentionsConditional(buf.String(), c.MentionEveryone, c.MentionHere, c.MentionRoles)
	if err != nil {
		return result, errors.WithMessage(err, "Failed executing template")
	}

	return result, nil
}

// IncreaseCheckCallCounter Returns true if key is above the limit
func (c *Context) IncreaseCheckCallCounter(key string, limit int) bool {
	current, ok := c.Counters[key]
	if !ok {
		current = 0
	}
	current++

	c.Counters[key] = current

	return current > limit
}

func baseContextFuncs(c *Context) {
	c.ContextFuncs["sendDM"] = c.tmplSendDM
	c.ContextFuncs["mentionEveryone"] = c.tmplMentionEveryone
	c.ContextFuncs["mentionHere"] = c.tmplMentionHere
	c.ContextFuncs["mentionRoleName"] = c.tmplMentionRoleName
	c.ContextFuncs["mentionRoleID"] = c.tmplMentionRoleID
	c.ContextFuncs["hasRoleName"] = c.tmplHasRoleName
	c.ContextFuncs["hasRoleID"] = c.tmplHasRoleID
	c.ContextFuncs["addRoleID"] = c.tmplAddRoleID
	c.ContextFuncs["removeRoleID"] = c.tmplRemoveRoleID
	c.ContextFuncs["deleteResponse"] = c.tmplDelResponse
	c.ContextFuncs["deleteTrigger"] = c.tmplDelTrigger
	c.ContextFuncs["addReactions"] = c.tmplAddReactions
}
