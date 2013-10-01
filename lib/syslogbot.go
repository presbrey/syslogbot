package syslogbot

import (
    "log"
    "os"
    "net"

    "io/ioutil"
    "encoding/json"

    "strings"
    "os/user"
    "math/big"
    "crypto/rand"
    "time"
    "regexp"

    "github.com/ziutek/syslog"
    "github.com/presbrey/go-ircevent"
)

type Config struct {
    Debug bool
    Default string
    Hide bool
    Hosts map[string]string
    Regex map[string]string
    Overflow uint
    Nick string
    Server string
    Password string
}

type syslogHandler struct {*syslog.BaseHandler}

type message struct {
    targets []string
    from string
    text string
}

type Server struct {
    config Config
    chdst chan string
    chmsg chan message
    overflow uint
    regex map[*regexp.Regexp]string
    syslogHandler *syslogHandler
    targets map[string]bool
    topics map[string]*regexp.Regexp
}

func NewBot(filename string) *Server {
    s := &Server{
        chdst: make(chan string, 32),
        chmsg: make(chan message, 32),
        syslogHandler: &syslogHandler{syslog.NewBaseHandler(5, func(m *syslog.Message)bool{return true}, false)},
        regex: make(map[*regexp.Regexp]string),
        targets: make(map[string]bool),
        topics: make(map[string]*regexp.Regexp),
    }
    s.load(filename)
    for _, target := range s.config.Hosts {
        s.targets[target] = true
    }
    for expr, target := range s.config.Regex {
        s.targets[target] = true
        if r, err := regexp.Compile(expr); err == nil {
            s.regex[r] = target
        } else { panic(err) }
    }
    if s.config.Overflow < 1 { s.config.Overflow = 100 }
    go s.readLoop()
    go s.writeLoop(s.setupIRC())
    return s
}

func (s *Server) SyslogHandler() *syslogHandler { return s.syslogHandler }

func (s *Server) load(filename string) {
    var err error
    var b []byte
    if b, err = ioutil.ReadFile(filename); err != nil { panic(err) }
    if err := json.Unmarshal(b, &s.config); err != nil { panic(err) }
}

func (s *Server) readLoop() {
    var (
        target string
        text string
        ex bool
    )
    for {
        target = ""
        m := s.syslogHandler.Get()
        if m == nil { break }
        if len(m.Content1) > 27 {
            text = m.Tag1 + " " + m.Content1[27:len(m.Content1)]
        } else {
            text = m.Tag1 + "|" + m.Content1
        }
        if text[len(text)-1] == '\n' { text = text[:len(text)-1] }
        host := m.Source.(*net.UDPAddr).IP.String()
        if target, ex = s.config.Hosts[host]; !ex {
            target = s.config.Default
        }
        for regex, v := range s.regex {
            if regex.MatchString(text) {
                target = v
                break
            }
        }
        if len(target) < 1 { continue }
        msg := message{targets: []string{target}, from: host, text: text}
        select {
        case s.chmsg <- msg:
        default:
            s.overflow++
            if s.overflow % s.config.Overflow == 0 {
                log.Printf("overflow threshold on: %+v\n", msg)
            }
        }
    }
    s.syslogHandler.End()
}

func (s *Server) setupIRC() *irc.Connection {
    if len(s.config.Nick) < 1 {
        hostname, _ := os.Hostname()
        s.config.Nick = strings.Replace(hostname, ".", "-", -1)
    }
    user, _ := user.Current()

    srv := irc.IRC(s.config.Nick, user.Username)
    srv.AddCallback("ERROR", func(e *irc.Event) { log.Println(e) })
    srv.AddCallback("001", func(e *irc.Event) {
        srv.SendRawf("MODE %s +D", e.Arguments[0])
    })
    srv.AddCallback("332", func(e *irc.Event) {
        if len(e.Message) < 1 { delete(s.topics, e.Arguments[1]); return }
        if r, err := regexp.Compile(e.Message); err == nil { s.topics[e.Arguments[1]] = r } else { log.Println(e.Message, err); }
    })
    srv.AddCallback("TOPIC", func(e *irc.Event) {
        if len(e.Message) < 1 { delete(s.topics, e.Arguments[0]); return }
        if r, err := regexp.Compile(e.Message); err == nil { s.topics[e.Arguments[0]] = r } else { log.Println(e.Message, err); }
    })
    srv.ReplaceCallback("433", 0, func(e *irc.Event) {
        max := big.NewInt(int64(1) << 31)
        bigx, _ := rand.Int(rand.Reader, max)
        srv.SendRawf("NICK %s_%x", s.config.Nick, bigx.Int64())
        if !s.config.Hide {
            srv.Join("#all")
            for dst := range s.targets { srv.Join(dst) }
        }
    })
    srv.VerboseCallbackHandler = s.config.Debug
    srv.Password = s.config.Password
    if err := srv.Connect(s.config.Server); err != nil { panic(err) }
    if !s.config.Hide {
        srv.Join("#all")
        for dst := range s.targets { srv.Join(dst) }
    }

    go func(irc *irc.Connection) {
        for {
            irc.Loop()
            time.Sleep(10 * time.Second)
        }
    }(srv)
    return srv
}

func (s *Server) writeLoop(irc *irc.Connection) {
    kill := make(chan bool)
    for {
        select {
        case msg := <-s.chmsg:
            for _, target := range msg.targets {
                filter, ex := s.topics[target]
                m := ex && (filter.MatchString(msg.from) || filter.MatchString(msg.text))
                if !ex || m { irc.Privmsgf(target, "[%s] %s", msg.from, msg.text) }
            }
        case dst := <-s.chdst:
            if _, ex := s.targets[dst]; !ex {
                s.targets[dst] = true
                if !s.config.Hide {
                    irc.Join(dst)
                }
            }
        case <-kill:
            return
        }
    }
}
