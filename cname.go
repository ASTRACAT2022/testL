package resolver

import (
	"context"
	"fmt"
	"github.com/miekg/dns"
	"log"
	"strings"
	"sync"
	"sync/atomic"
)

var cnameCallCount uint64

func cname(ctx context.Context, qmsg *dns.Msg, r *Response, exchanger exchanger, cache *DNSCache) error {
	atomic.AddUint64(&cnameCallCount, 1)
	log.Printf("CNAME function called %d times", atomic.LoadUint64(&cnameCallCount))
	cnames := extractRecords[*dns.CNAME](r.Msg.Answer)

	targets := make([]string, len(cnames))
	for i, c := range cnames {
		targets[i] = c.Target
	}

	Debug(fmt.Sprintf("resolved [%s]  to cnames: [%s]",
		qmsg.Question[0].Name,
		strings.Join(targets, ", ")),
	)

	var wg sync.WaitGroup
	// Канал для сбора результатов из горутин
	cnameResponses := make(chan *Response, len(cnames))
	cnameErrors := make(chan error, len(cnames))

	for _, c := range cnames {
		wg.Add(1)
		go func(c *dns.CNAME) {
			defer wg.Done()

			target := dns.CanonicalName(c.Target)

			if recordsOfNameAndTypeExist(r.Msg.Answer, target, qmsg.Question[0].Qtype) || recordsOfNameAndTypeExist(r.Msg.Answer, target, dns.TypeCNAME) {
				// Skip over if the answer already contains a record for the target.
				return
			}

			cnameQMsg := new(dns.Msg)
			cnameQMsg.SetQuestion(target, qmsg.Question[0].Qtype)

			var cnameRMsg *Response
			// Проверяем кэш для CNAME-записи
			if cachedMsg := cache.get(cnameQMsg.Question[0], qmsg.Id); cachedMsg != nil {
				cnameRMsg = &Response{Msg: cachedMsg}
			} else {
				if isSetDO(qmsg) {
					cnameQMsg.SetEdns0(4096, true)
				}
				cnameRMsg = exchanger.exchange(ctx, cnameQMsg)
				// Кэшируем ответ, если он не содержит ошибок
				if !cnameRMsg.HasError() {
					cache.set(cnameQMsg.Question[0], cnameRMsg.Msg)
				}
			}

			if cnameRMsg.HasError() {
				cnameErrors <- cnameRMsg.Err
				return
			}
			if cnameRMsg.IsEmpty() {
				cnameErrors <- fmt.Errorf("unable to follow cname [%s]", c.Target)
				return
			}
			cnameResponses <- cnameRMsg
		}(c)
	}

	wg.Wait()
	close(cnameResponses)
	close(cnameErrors)

	var allErrors []error
	for err := range cnameErrors {
		allErrors = append(allErrors, err)
	}

	hasSuccessfulResponse := false
	for cnameRMsg := range cnameResponses {
		hasSuccessfulResponse = true
		r.Msg.Answer = append(r.Msg.Answer, cnameRMsg.Msg.Answer...)
		r.Msg.Ns = append(r.Msg.Ns, cnameRMsg.Msg.Ns...)
		r.Msg.Extra = append(r.Msg.Extra, cnameRMsg.Msg.Extra...)

		// Ensure we handle differing DNSSEC results correctly.
		r.Auth = r.Auth.Combine(cnameRMsg.Auth)

		// The overall message is only authoritative if all answers are.
		r.Msg.Authoritative = r.Msg.Authoritative && cnameRMsg.Msg.Authoritative

		// Ensures we don't return 0 if any message was not 0. TODO: should this be more sophisticated?
		r.Msg.Rcode = max(r.Msg.Rcode, cnameRMsg.Msg.Rcode)
	}

	if !hasSuccessfulResponse && len(allErrors) > 0 {
		return fmt.Errorf("all cname resolutions failed: %v", allErrors)
	}

	return nil
}
