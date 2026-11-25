package stdlib

import (
	"strings"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/xmaps"
)

type SLogPass struct{}

func (s *SLogPass) Name() string {
	return "slog"
}

func (s *SLogPass) findTag(svc *di.Service) (di.ServiceTag, bool) {
	for _, tag := range svc.Tags {
		if strings.EqualFold(tag.Name, s.Name()) {
			return tag, true
		}
	}

	return di.ServiceTag{}, false
}

func (s *SLogPass) Process(cfg *di.Config) (*di.Config, error) {
	// The loop adds services to the map it walks. A snapshot of the sorted
	// keys makes the result independent of the iteration order.
	for _, id := range xmaps.OrderedKeys(cfg.Services) {
		svc := cfg.Services[id]
		tag, ok := s.findTag(&svc)
		if !ok {
			continue
		}

		channelName, ok := tag.Attributes["channel"]
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
		if _, exists := cfg.Services[namedLoggerSvcName]; exists {
			loc := tag.SourceLoc
			if loc == nil {
				loc = svc.SourceLoc
			}
			return nil, srcloc.Errorf(loc, "service %q: the slog tag adds the service %q, but the config already has a service with this ID", id, namedLoggerSvcName)
		}

		newArgs := make([]di.Argument, 0, len(svc.Constructor.Args))
		for _, arg := range svc.Constructor.Args {
			if arg.Kind == di.ArgServiceRef && arg.Value == "logger" {
				arg = di.Argument{
					Kind:  di.ArgServiceRef,
					Value: namedLoggerSvcName,
				}
			}

			newArgs = append(newArgs, arg)
		}
		svc.Constructor.Args = newArgs

		cfg.Services[id] = svc
		cfg.Services[namedLoggerSvcName] = namedLoggerSvc
	}

	return cfg, nil
}
