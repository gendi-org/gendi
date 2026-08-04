package stdlib

import (
	"slices"
	"strings"

	di "github.com/gendi-org/gendi"
)

type SLogPass struct{}

func (s *SLogPass) Name() string {
	return "slog"
}

func (s *SLogPass) getTagAttributes(svc *di.Service) (map[string]any, bool) {
	for _, tag := range svc.Tags {
		if !strings.EqualFold(tag.Name, s.Name()) {
			continue
		}

		return tag.Attributes, true
	}

	return nil, false
}

func (s *SLogPass) Process(cfg *di.Config) (*di.Config, error) {
	for id, svc := range cfg.Services {
		tagAttributes, ok := s.getTagAttributes(&svc)
		if !ok {
			continue
		}

		channelName, ok := tagAttributes["channel"]
		if !ok {
			continue
		}
		channelNameStr, ok := channelName.(string)
		if !ok {
			continue
		}

		namedLoggerSvc := di.Service{
			Constructor: di.Constructor{
				Method: "@logger.With",
				Args: []di.Argument{
					{Kind: di.ArgLiteral, Literal: di.NewStringLiteral("channel")},
					{Kind: di.ArgLiteral, Literal: di.NewStringLiteral(channelNameStr)},
				},
			},
			Shared: false,
		}
		namedLoggerSvcName := id + ".logger"

		newArgs := slices.Clone(svc.Constructor.Args)
		for i := range newArgs {
			rewriteLoggerRef(&newArgs[i], namedLoggerSvcName)
		}
		svc.Constructor.Args = newArgs

		cfg.Services[id] = svc
		cfg.Services[namedLoggerSvcName] = namedLoggerSvc
	}

	return cfg, nil
}

// rewriteLoggerRef repoints @logger at the channel-specific logger service,
// including references nested inside a map argument.
func rewriteLoggerRef(arg *di.Argument, namedLoggerSvcName string) {
	if arg.Kind == di.ArgServiceRef && arg.Value == "logger" {
		arg.Value = namedLoggerSvcName
	}
	for _, child := range arg.Children() {
		rewriteLoggerRef(child, namedLoggerSvcName)
	}
}
