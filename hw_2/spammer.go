package main

import (
	"fmt"
	"log"
	"sort"
	"sync"
)

func RunPipeline(cmds ...cmd) {
	if len(cmds) == 0 {
		return
	}

	var wg sync.WaitGroup
	in := make(chan interface{})

	for _, c := range cmds {
		out := make(chan interface{})

		wg.Add(1)
		go func(c cmd, in, out chan interface{}) {
			defer wg.Done()
			defer close(out)
			c(in, out)
		}(c, in, out)

		in = out
	}

	wg.Wait()
}

func SelectUsers(in, out chan interface{}) {
	var wg sync.WaitGroup

	seenEmails := make(map[string]struct{})

	seenUsers := make(map[string]struct{})

	var mu sync.Mutex

	for email := range in {
		e, ok := email.(string)
		if !ok {
			continue
		}

		if _, exists := seenEmails[e]; exists {
			continue
		}
		seenEmails[e] = struct{}{}

		wg.Add(1)
		go func(email string) {
			defer wg.Done()

			user := GetUser(email)

			mu.Lock()
			if _, exists := seenUsers[user.Email]; exists {
				mu.Unlock()
				return
			}
			seenUsers[user.Email] = struct{}{}
			mu.Unlock()

			out <- user
		}(e)
	}

	wg.Wait()
}

func SelectMessages(in, out chan interface{}) {
	// 	in - User
	// 	out - MsgID
	batch := make([]User, 0, GetMessagesMaxUsersBatch)
	var wg sync.WaitGroup

	flush := func() {
		if len(batch) == 0 {
			return
		}

		users := make([]User, len(batch))
		copy(users, batch)
		batch = batch[:0]

		wg.Add(1)
		go func(users []User) {
			defer wg.Done()

			msgs, err := GetMessages(users...)
			if err != nil {
				return
			}

			for _, msg := range msgs {
				out <- msg
			}
		}(users)
	}

	for curr := range in {
		u, ok := curr.(User)
		if !ok {
			continue
		}

		batch = append(batch, u)
		if len(batch) >= GetMessagesMaxUsersBatch {
			flush()
		}
	}

	flush()
	wg.Wait()
}

func CheckSpam(in, out chan interface{}) {
	// in - MsgID
	// out - MsgData
	var wg sync.WaitGroup

	ch := make(chan struct{}, HasSpamMaxAsyncRequests)

	for v := range in {
		id, ok := v.(MsgID)
		if !ok {
			continue
		}

		wg.Add(1)
		go func(id MsgID) {
			defer wg.Done()

			ch <- struct{}{} // занимаем слот
			defer func() { <-ch }()

			hasSpam, err := HasSpam(id)
			if err != nil {
				log.Printf("HasSpam error for %d: %v", id, err)
				return
			}

			out <- MsgData{id, hasSpam}
		}(id)
	}

	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	var results []MsgData
	for v := range in {
		msgData, ok := v.(MsgData)
		if !ok {
			continue
		}

		results = append(results, msgData)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].HasSpam != results[j].HasSpam {
			return results[i].HasSpam && !results[j].HasSpam
		}

		return results[i].ID < results[j].ID
	})

	for _, result := range results {
		out <- fmt.Sprintf("%v %d", result.HasSpam, result.ID)
	}
}
