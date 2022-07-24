package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fiatjaf/go-nostr"
)

func (b *ClosedRelay) SaveEvent(evt *nostr.Event) error {
	// disallow anything from non-authorized pubkeys
	for _, pubkey := range b.AuthorizedPubkeys {
		if pubkey == evt.PubKey {
			goto save
		}
	}
	return fmt.Errorf("event from '%s' not allowed here", evt.PubKey)

save:
	// react to different kinds of events
	if evt.Kind == nostr.KindSetMetadata || evt.Kind == nostr.KindContactList || (10000 <= evt.Kind && evt.Kind < 20000) {
		// delete past events from this user
		b.DB.Exec(`DELETE FROM event WHERE pubkey = $1 AND kind = $2`, evt.PubKey, evt.Kind)
	} else if evt.Kind == nostr.KindRecommendServer {
		// delete past recommend_server events equal to this one
		b.DB.Exec(`DELETE FROM event WHERE pubkey = $1 AND kind = $2 AND content = $3`,
			evt.PubKey, evt.Kind, evt.Content)
	} else {
		// do not delete any, this is a closed relay so we trust everybody to not spam
	}

	// insert
	tagsj, _ := json.Marshal(evt.Tags)
	_, err := b.DB.Exec(`
        INSERT INTO event (id, pubkey, created_at, kind, tags, content, sig)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, evt.ID, evt.PubKey, evt.CreatedAt.Unix(), evt.Kind, tagsj, evt.Content, evt.Sig)
	if err != nil {
		if strings.Index(err.Error(), "UNIQUE") != -1 {
			// already exists
			return nil
		}

		return fmt.Errorf("failed to save event %s: %w", evt.ID, err)
	}

	// delete ephemeral events after a minute
	go func() {
		time.Sleep(75 * time.Second)
		b.DB.Exec("DELETE FROM event WHERE id = $1", evt.ID)
	}()

	return nil
}
