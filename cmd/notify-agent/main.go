// notify-agent는 맥의 알림 센터에 온 알림을 relay-service로 넘긴다.
//
// 아이폰 앱은 다른 앱의 알림을 읽을 수 없고, 안경은 Relay 플러그인이 켜져
// 있으면 시스템 알림을 가린다. 그래서 맥에 뜬 알림(카카오톡 맥 버전, 슬랙,
// 메일…)을 여기서 읽어 relay-service 알림으로 올린다. 그러면 Relay 알림
// 목록·안경 팝업·폰 앱에 뜬다.
//
// 알림 기록을 읽으려면 '전체 디스크 접근' 권한이 필요하다. 셸을 여는
// terminal-agent에 그 권한을 주지 않으려고 따로 뗐다. 이 프로그램은
// 알림 기록을 읽고 훅 하나로 보내는 것밖에 하지 않는다.
//
//	notify-agent          알림을 지켜보며 보낸다
//	notify-agent apps     알림을 보낸 앱과 수를 보인다(가져올 앱 고르기)
//	notify-agent test     relay-service에 시험 알림을 하나 보낸다
//	notify-agent version
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/foncdev/notify-agent/internal/appname"
	"github.com/foncdev/notify-agent/internal/config"
	"github.com/foncdev/notify-agent/internal/ncdb"
	"github.com/foncdev/notify-agent/internal/relay"
	"github.com/foncdev/notify-agent/internal/state"
)

var version = "dev"

func main() {
	envFile := flag.String("env", config.DefaultEnvFile(), "설정 파일")
	flag.Parse()
	log.SetFlags(log.LstdFlags)

	cfg, err := config.Load(*envFile)
	if err != nil {
		log.Fatal(err)
	}
	reader := ncdb.Reader{Path: cfg.DBPath}

	switch flag.Arg(0) {
	case "", "run":
		err = run(cfg, reader)
	case "apps":
		err = listApps(reader)
	case "test":
		err = client(cfg).Send(relay.Message{Title: "[notify-agent] 시험 알림", Body: "맥 알림을 relay-service로 넘길 수 있습니다."})
		if err == nil {
			fmt.Println("보냈습니다. 안경·폰의 알림 목록을 보세요.")
		}
	case "version":
		fmt.Println(version)
	default:
		err = fmt.Errorf("모르는 명령: %s (run·apps·test·version)", flag.Arg(0))
	}
	if err != nil {
		log.Fatal(err)
	}
}

func client(cfg config.Config) relay.Client {
	return relay.Client{BaseURL: cfg.RelayURL, HookKey: cfg.HookKey, Source: "mac"}
}

func listApps(reader ncdb.Reader) error {
	apps, err := reader.Apps()
	if err != nil {
		return err
	}
	fmt.Println("알림 수  번들 id  (앱 이름)")
	for _, a := range apps {
		fmt.Printf("%7d  %s  (%s)\n", a.Count, a.App, appname.Of(a.App))
	}
	fmt.Println("\n가져올 앱의 번들 id를 NOTIFY_APPS에 쉼표로 적으세요.")
	return nil
}

func run(cfg config.Config, reader ncdb.Reader) error {
	if cfg.HookKey == "" {
		return errors.New("NOTIFY_HOOK_KEY가 없습니다. relay-service의 RELAY_HOOK_KEY를 적으세요")
	}
	if len(cfg.Apps) == 0 {
		log.Print("NOTIFY_APPS가 비어 있어 아무 알림도 보내지 않습니다. `notify-agent apps`로 보고 정하세요.")
	}

	st, ok, err := state.Load(cfg.StatePath)
	if err != nil {
		return err
	}
	if !ok {
		// 처음이다. 지난 알림을 한꺼번에 보내지 않도록 지금부터 센다.
		last, err := reader.LastID()
		if err != nil {
			return err
		}
		st.LastID = last
		if err := state.Save(cfg.StatePath, st); err != nil {
			return err
		}
	}
	log.Printf("notify-agent %s 시작: %s 로 보냄, 앱 %s, %s마다", version, cfg.RelayURL, strings.Join(cfg.Apps, ","), cfg.Poll)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	tick := time.NewTicker(cfg.Poll)
	defer tick.Stop()

	var lastErr string
	for {
		if next, err := forward(cfg, reader, st.LastID); err != nil {
			// 같은 오류를 5초마다 되풀이해 로그를 채우지 않는다.
			if err.Error() != lastErr {
				log.Print(err)
				lastErr = err.Error()
			}
			if next != st.LastID {
				st.LastID = next
				_ = state.Save(cfg.StatePath, st)
			}
		} else {
			lastErr = ""
			if next != st.LastID {
				st.LastID = next
				if err := state.Save(cfg.StatePath, st); err != nil {
					log.Print(err)
				}
			}
		}
		select {
		case <-stop:
			log.Print("끝냅니다.")
			return nil
		case <-tick.C:
		}
	}
}

// forward는 last 뒤의 알림 중 허락한 앱의 것을 보낸다. 어디까지 처리했는지 돌려준다.
// 보내다 실패하면 거기서 멈춘다 — 다음 번에 그 알림부터 다시 보낸다.
func forward(cfg config.Config, reader ncdb.Reader, last int64) (int64, error) {
	items, err := reader.After(last)
	if err != nil {
		return last, err
	}
	c := client(cfg)
	for _, n := range items {
		if cfg.Allows(n.App) {
			if msg, ok := Format(n, appname.Of(n.App)); ok {
				if err := c.Send(msg); err != nil {
					return last, err
				}
			}
		}
		last = n.ID
	}
	return last, nil
}

// Format은 알림 하나를 relay-service 알림으로 바꾼다. 글이 없으면 보내지 않는다.
//
// 제목 앞에 앱 이름을 붙인다. 안경 팝업은 좁아 어디서 온 것인지 먼저 보여야 한다.
func Format(n ncdb.Notification, app string) (relay.Message, bool) {
	title := strings.TrimSpace(n.Title)
	body := strings.TrimSpace(strings.Join(nonEmpty(n.Subtitle, n.Body), "\n"))
	if title == "" && body == "" {
		return relay.Message{}, false
	}
	if title == "" {
		title = app
	} else {
		title = "[" + app + "] " + title
	}
	return relay.Message{Title: title, Body: body}, true
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
