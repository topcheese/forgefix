package engine

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"
)

func (d *CommandDispatcher) handleKanban(args []string) (CommandResult, error) {
	db, err := OpenDB(d.ConfigDir)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: opening database: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	defer db.Close()
	if len(args) == 0 {
		return d.kanbanList(db)
	}
	cmd := args[0]
	switch cmd {
	case "init":
		return d.kanbanInit(db)
	case "column", "col":
		return d.kanbanColumn(db, args[1:])
	case "card":
		return d.kanbanCard(db, args[1:])
	case "ls", "list":
		return d.kanbanList(db)
	case "ui", "watch":
		return d.kanbanWatch(db)
	default:
		fmt.Fprintf(d.Stderr, "error: unknown kanban command %q\n", cmd)
		fmt.Fprintln(d.Stderr, "Usage: ff --kanban <init|column|card|ls|ui>")
		return CommandResult{ExitCode: 1}, nil
	}
}

func (d *CommandDispatcher) kanbanInit(db *DB) (CommandResult, error) {
	boardID, err := db.InitDefaultBoard()
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	fmt.Fprintf(d.Stdout, "Kanban board initialized (id: %s)\n", boardID)
	return CommandResult{ExitCode: 0}, nil
}

func (d *CommandDispatcher) kanbanColumn(db *DB, args []string) (CommandResult, error) {
	if len(args) < 1 {
		fmt.Fprintln(d.Stderr, "usage: ff --kanban column <new|ls> [title]")
		return CommandResult{ExitCode: 1}, nil
	}
	boards, err := db.ListBoards()
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	if len(boards) == 0 {
		fmt.Fprintln(d.Stderr, "error: no board exists. Run 'ff --kanban init' first.")
		return CommandResult{ExitCode: 1}, nil
	}
	switch args[0] {
	case "new":
		if len(args) < 2 {
			fmt.Fprintln(d.Stderr, "usage: ff --kanban column new <title>")
			return CommandResult{ExitCode: 1}, nil
		}
		title := strings.Join(args[1:], " ")
		if err := db.CreateColumn(boards[0].ID, title); err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		fmt.Fprintf(d.Stdout, "Column %q created.\n", title)
		return CommandResult{ExitCode: 0}, nil
	case "ls":
		board, err := db.ListBoard(boards[0].ID)
		if err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		for _, col := range board.Columns {
			fmt.Fprintf(d.Stdout, "  %s (%s, pos %d)\n", col.Title, col.ID, col.Position)
		}
		return CommandResult{ExitCode: 0}, nil
	default:
		fmt.Fprintln(d.Stderr, "usage: ff --kanban column <new|ls> [title]")
		return CommandResult{ExitCode: 1}, nil
	}
}

func (d *CommandDispatcher) kanbanCard(db *DB, args []string) (CommandResult, error) {
	if len(args) < 1 {
		fmt.Fprintln(d.Stderr, "usage: ff --kanban card <add|move|delete> ...")
		return CommandResult{ExitCode: 1}, nil
	}
	switch args[0] {
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(d.Stderr, "usage: ff --kanban card add <spec_id|title>")
			return CommandResult{ExitCode: 1}, nil
		}
		title := strings.Join(args[1:], " ")
		boards, err := db.ListBoards()
		if err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		if len(boards) == 0 {
			fmt.Fprintln(d.Stderr, "error: no board exists. Run 'ff --kanban init' first.")
			return CommandResult{ExitCode: 1}, nil
		}
		board, err := db.ListBoard(boards[0].ID)
		if err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		if len(board.Columns) == 0 {
			fmt.Fprintln(d.Stderr, "error: board has no columns.")
			return CommandResult{ExitCode: 1}, nil
		}
		cardID, err := db.CreateCard(board.Columns[0].ID, "spec", title)
		if err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		fmt.Fprintf(d.Stdout, "Card %q added to column %q (id: %s)\n", title, board.Columns[0].Title, cardID)
		return CommandResult{ExitCode: 0}, nil
	case "move":
		if len(args) < 3 {
			fmt.Fprintln(d.Stderr, "usage: ff --kanban card move <card_id> <column_id>")
			return CommandResult{ExitCode: 1}, nil
		}
		if err := db.MoveCard(args[1], args[2]); err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		fmt.Fprintf(d.Stdout, "Card %s moved to column %s.\n", args[1], args[2])
		return CommandResult{ExitCode: 0}, nil
	case "delete", "rm":
		if len(args) < 2 {
			fmt.Fprintln(d.Stderr, "usage: ff --kanban card delete <card_id>")
			return CommandResult{ExitCode: 1}, nil
		}
		if err := db.DeleteCard(args[1]); err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		fmt.Fprintf(d.Stdout, "Card %s deleted.\n", args[1])
		return CommandResult{ExitCode: 0}, nil
	default:
		fmt.Fprintln(d.Stderr, "usage: ff --kanban card <add|move|delete> ...")
		return CommandResult{ExitCode: 1}, nil
	}
}

func (d *CommandDispatcher) kanbanList(db *DB) (CommandResult, error) {
	if err := db.SyncCards(d.ConfigDir); err != nil {
		fmt.Fprintf(d.Stderr, "warning: card sync failed: %v\n", err)
	}
	boards, err := db.ListBoards()
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	if len(boards) == 0 {
		fmt.Fprintln(d.Stdout, "No boards. Run 'ff --kanban init' to create one.")
		return CommandResult{ExitCode: 0}, nil
	}
	w := tabwriter.NewWriter(d.Stdout, 0, 8, 1, '\t', 0)
	for _, b := range boards {
		fmt.Fprintf(w, "Board:\t%s\t(%s)\n", b.Name, b.ID)
		board, err := db.ListBoard(b.ID)
		if err != nil {
			continue
		}
		for _, col := range board.Columns {
			extra := ""
			if col.Title == "In Progress" {
				stats, err := db.GetAllPipelineStats()
				if err == nil && len(stats) > 0 {
					s := stats[0]
					extra = fmt.Sprintf("[tests: %d/%d pass, %d fail]", s.TotalPassed, s.TotalRan, s.TotalFailed)
				}
			}
			fmt.Fprintf(w, "  %s:\t%s\n", col.Title, extra)
			for _, card := range col.Cards {
				statusInfo := ""
				if strings.HasPrefix(card.Title, "SPEC-") || strings.HasPrefix(card.Title, "SPEC ") {
					specID := strings.Fields(card.Title)[0]
					if ledger, lErr := LoadLedger(d.ConfigDir); lErr == nil {
						if entry := ledger.GetSpecEntry(specID); entry != nil {
							statusInfo = fmt.Sprintf("[spec: %s]", entry.Status)
						}
					}
				}
				fmt.Fprintf(w, "    -\t[%s]\t%s\t%s\n", card.Status, card.Title, statusInfo)
			}
		}
	}
	w.Flush()
	return CommandResult{ExitCode: 0}, nil
}

func (d *CommandDispatcher) kanbanWatch(db *DB) (CommandResult, error) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return d.kanbanList(db)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)
	fmt.Fprint(d.Stdout, "\033[?25l")
	defer fmt.Fprint(d.Stdout, "\033[?25h")

	keyCh := make(chan byte, 1)
	go func() {
		b := make([]byte, 1)
		for {
			if _, err := os.Stdin.Read(b); err != nil {
				return
			}
			keyCh <- b[0]
		}
	}()

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	db.SyncCards(d.ConfigDir)

	render := func() {
		fmt.Fprint(d.Stdout, "\033[H\033[2J")
		db.SyncCards(d.ConfigDir)
		d.kanbanList(db)
	}

	render()

	for {
		select {
		case key := <-keyCh:
			if key == 'q' || key == 'Q' {
				return CommandResult{ExitCode: 0}, nil
			}
			if key == 'r' || key == 'R' {
				render()
			}
		case <-tick.C:
			render()
		}
	}
}
