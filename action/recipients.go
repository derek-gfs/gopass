package action

import (
	"context"
	"fmt"
	"strings"

	"github.com/justwatchcom/gopass/utils/ctxutil"
	"github.com/justwatchcom/gopass/utils/cui"
	"github.com/justwatchcom/gopass/utils/out"
	"github.com/justwatchcom/gopass/utils/termio"
	"github.com/urfave/cli"
)

var (
	removalWarning = `


@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@ WARNING: REMOVING A USER WILL NOT REVOKE ACCESS FROM OLD REVISONS!     @
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
THE USER %s WILL STILL BE ABLE TO ACCESS ANY OLD COPY OF THE STORE AND
ANY OLD REVISION HE HAD ACCESS TO.

ANY CREDENTIALS THIS USER HAD ACCESS TO NEED TO BE CONSIDERED COMPROMISED
AND SHOULD BE REVOKED.

This feature is only meant from revoking access to any added or changed
credentials.

`
)

// RecipientsPrint prints all recipients per store
func (s *Action) RecipientsPrint(ctx context.Context, c *cli.Context) error {
	out.Cyan(ctx, "Hint: run 'gopass sync' to import any missing public keys")

	tree, err := s.Store.RecipientsTree(ctx, true)
	if err != nil {
		return exitError(ctx, ExitList, err, "failed to list recipients: %s", err)
	}

	fmt.Fprintln(stdout, tree.Format(0))
	return nil
}

// RecipientsComplete will print a list of recipients for bash
// completion
func (s *Action) RecipientsComplete(ctx context.Context, c *cli.Context) {
	tree, err := s.Store.RecipientsTree(ctx, false)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return
	}

	for _, v := range tree.List(0) {
		fmt.Fprintln(stdout, v)
	}
}

// RecipientsAdd adds new recipients
func (s *Action) RecipientsAdd(ctx context.Context, c *cli.Context) error {
	store := c.String("store")
	added := 0

	// select store
	if store == "" {
		store = s.askForStore(ctx)
	}

	crypto := s.Store.Crypto(ctx, store)

	// select recipient
	recipients := []string(c.Args())
	if len(recipients) < 1 {
		choices := []string{}
		kl, _ := crypto.FindPublicKeys(ctx)
		for _, key := range kl {
			choices = append(choices, crypto.FormatKey(ctx, key))
		}
		if len(choices) > 0 {
			act, sel := cui.GetSelection(ctx, "Add Recipient -", "<↑/↓> to change the selection, <→> to add this recipient, <ESC> to quit", choices)
			switch act {
			case "default":
				fallthrough
			case "show":
				recipients = []string{kl[sel]}
			default:
				return exitError(ctx, ExitAborted, nil, "user aborted")
			}
		}
	}

	for _, r := range recipients {
		keys, err := crypto.FindPublicKeys(ctx, r)
		if err != nil {
			out.Cyan(ctx, "Failed to list public key '%s': %s", r, err)
			continue
		}
		if len(keys) < 1 {
			out.Cyan(ctx, "Warning: No matching valid key found. If the key is in your keyring you may need to validate it.")
			out.Cyan(ctx, "If this is your key: gpg --edit-key %s; trust (set to ultimate); quit", r)
			out.Cyan(ctx, "If this is not your key: gpg --edit-key %s; lsign; trust; save; quit", r)
			out.Cyan(ctx, "You may need to run 'gpg --update-trustdb' afterwards")
			continue
		}

		if !termio.AskForConfirmation(ctx, fmt.Sprintf("Do you want to add '%s' as an recipient to the store '%s'?", crypto.FormatKey(ctx, keys[0]), store)) {
			continue
		}

		if err := s.Store.AddRecipient(ctxutil.WithNoConfirm(ctx, true), store, keys[0]); err != nil {
			return exitError(ctx, ExitRecipients, err, "failed to add recipient '%s': %s", r, err)
		}
		added++
	}
	if added < 1 {
		return exitError(ctx, ExitUnknown, nil, "no key added")
	}

	out.Green(ctx, "\nAdded %d recipients", added)
	out.Cyan(ctx, "You need to run 'gopass sync' to push these changes")
	return nil
}

// RecipientsRemove removes recipients
func (s *Action) RecipientsRemove(ctx context.Context, c *cli.Context) error {
	store := c.String("store")

	// select store
	if store == "" {
		store = s.askForStore(ctx)
	}

	crypto := s.Store.Crypto(ctx, store)

	// select recipient
	recipients := []string(c.Args())
	if len(recipients) < 1 {
		rs, err := s.recipientsSelectForRemoval(ctx, store)
		if err != nil {
			return err
		}
		recipients = rs
	}

	removed := 0
	for _, r := range recipients {
		kl, err := crypto.FindPrivateKeys(ctx, r)
		if err == nil {
			if len(kl) > 0 {
				if !termio.AskForConfirmation(ctx, fmt.Sprintf("Do you want to remove yourself (%s) from the recipients?", r)) {
					continue
				}
			}
		}
		if err := s.Store.RemoveRecipient(ctxutil.WithNoConfirm(ctx, true), store, strings.TrimPrefix(r, "0x")); err != nil {
			return exitError(ctx, ExitRecipients, err, "failed to remove recipient '%s': %s", r, err)
		}
		fmt.Fprintf(stdout, removalWarning, r)
		removed++
	}

	out.Green(ctx, "\nRemoved %d recipients", removed)
	out.Cyan(ctx, "You need to run 'gopass sync' to push these changes")
	return nil
}

func (s *Action) recipientsSelectForRemoval(ctx context.Context, store string) ([]string, error) {
	crypto := s.Store.Crypto(ctx, store)

	ids := s.Store.ListRecipients(ctx, store)
	choices := make([]string, 0, len(ids))
	for _, id := range ids {
		choices = append(choices, crypto.FormatKey(ctx, id))
	}
	if len(choices) < 1 {
		return nil, nil
	}
	act, sel := cui.GetSelection(ctx, "Remove recipient -", "<↑/↓> to change the selection, <→> to remove this recipient, <ESC> to quit", choices)
	switch act {
	case "default":
		fallthrough
	case "show":
		return []string{ids[sel]}, nil
	default:
		return nil, exitError(ctx, ExitAborted, nil, "user aborted")
	}
}
