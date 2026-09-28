# notify-agent

맥의 알림 센터에 온 알림(카카오톡 맥 버전, 슬랙, 메일 등)을 relay-service로 넘긴다.
넘긴 알림은 Relay 알림 목록·안경 팝업·폰 앱에 뜬다.

## 왜 따로 있나

- 아이폰 앱은 다른 앱의 알림을 읽을 수 없다(iOS가 막는다).
- 안경은 Relay 플러그인이 켜져 있으면 시스템 알림(카카오톡·전화)을 가린다.
- 맥의 알림 기록은 읽을 수 있지만 **전체 디스크 접근** 권한이 필요하다.
  셸을 여는 terminal-agent에 그 권한을 주면 위험하므로, 알림만 읽는 이 프로그램을 따로 둔다.

이 프로그램이 하는 일은 알림 기록을 읽는 것과, relay-service 알림 훅(`POST /hooks/notify/mac`)으로
보내는 것 두 가지뿐이다. 훅 키는 알림 추가만 할 수 있다.

> 알림 설정·문제 해결까지 자세한 안내는 [docs/알림-설정-가이드.md](docs/알림-설정-가이드.md)를 본다.

## 설치

```sh
make install
```

설치하면 `~/.local/bin/notify-agent`에 두고, 로그인할 때마다 켜지게 한다(LaunchAgent).
처음 설치할 때 훅 키를 만들어 `~/.config/notify-agent/env`에 적고, relay-service에 넣을 줄을 보여 준다.

그다음 한 번은 사람이 해야 한다.

1. **relay-service에 훅 키 넣기.** relay-service `.env`에 설치 때 보여 준 `RELAY_HOOK_KEY=…`를 넣고 다시 켠다.
2. **전체 디스크 접근 주기.** 시스템 설정 > 개인정보 보호 및 보안 > 전체 디스크 접근에서 `+`를 누르고
   `~/.local/bin/notify-agent`를 고른다(Finder에서 `⌘⇧G`로 경로 입력). 켜 두면 10초 안에 스스로 읽기 시작한다.
3. **가져올 앱 고르기.**
   ```sh
   ~/.local/bin/notify-agent apps
   ```
   알림을 보낸 앱과 번들 id가 나온다. 가져올 것을 `~/.config/notify-agent/env`의 `NOTIFY_APPS`에 쉼표로 적고
   `make install`을 다시 하거나 `launchctl kickstart -k gui/$(id -u)/com.foncsoft.notify-agent`로 다시 켠다.
   비워 두면 아무것도 보내지 않는다 — 개인 메시지가 서버에 쌓이므로 사람이 골라야 한다.

시험:

```sh
~/.local/bin/notify-agent test   # relay-service에 시험 알림 하나
tail -f ~/Library/Logs/notify-agent.log
```

## 설정

`~/.config/notify-agent/env` (환경 변수가 있으면 그쪽이 먼저)

| 이름 | 기본 | 뜻 |
|---|---|---|
| `NOTIFY_RELAY_URL` | `http://127.0.0.1:4100` | relay-service 주소 |
| `NOTIFY_HOOK_KEY` | (없음) | relay-service의 `RELAY_HOOK_KEY` |
| `NOTIFY_APPS` | (없음) | 가져올 앱의 번들 id, 쉼표로. `*`는 전부 |
| `NOTIFY_POLL` | `5s` | 알림 기록을 들여다보는 간격 |
| `NOTIFY_STATE` | `~/Library/Application Support/notify-agent/state.json` | 어디까지 보냈는지 |
| `NOTIFY_DB` | macOS 기본 자리 | 알림 기록 파일 |

## 동작

- 처음 켜면 그때까지 쌓인 알림은 건너뛰고 새로 오는 것부터 보낸다.
- 어디까지 보냈는지 적어 두어, 다시 켜도 같은 알림을 두 번 보내지 않고 꺼져 있던 사이 온 것은 보낸다.
- relay-service가 받지 않으면 거기서 멈췄다가 다음에 그 알림부터 다시 보낸다.
- 알림 제목 앞에 앱 이름을 붙인다: `[카카오톡] 엄마`.
- 알림 기록은 usernoted가 쓰는 중인 파일이라, 매번 임시 폴더에 복사해 그 복사본을 읽는다.

## 주의

- 알림 기록(`~/Library/Group Containers/group.com.apple.usernoted/db2/db`)은 애플이 공개한 형식이 아니다.
  macOS를 올리면 자리나 모양이 바뀌어 멈출 수 있다. 그러면 로그에 "모양이 예상과 다릅니다"가 남는다.
- 바이너리를 새로 설치하면 서명이 바뀌어 전체 디스크 접근을 다시 줘야 할 수 있다.
- 맥이 켜져 있어야 한다.

## 개발

```sh
make check   # go vet + 테스트
make build
```

테스트는 macOS 알림 기록과 같은 모양의 SQLite를 만들어 읽는다. 실제 기록은 건드리지 않는다.

## 지우기

```sh
make uninstall
```
