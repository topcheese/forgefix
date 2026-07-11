package engine

import (
	"fmt"
	"strings"
)

// handleKanban routes --kanban subcommands.
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
	default:
		fmt.Fprintf(d.Stderr, "error: unknown kanban command %q\n", cmd)
		fmt.Fprintln(d.Stderr, "Usage: ff --kanban <init|column|card|ls>")
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
	if len(args) < 2 {
		fmt.Fprintln(d.Stderr, "usage: ff --kanban column new <title>")
		return CommandResult{ExitCode: 1}, nil
	}
	switch args[0] {
	case "new":
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
		if err := db.CreateColumn(boards[0].ID, title); err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		fmt.Fprintf(d.Stdout, "Column %q created.\n", title)
		return CommandResult{ExitCode: 0}, nil
	default:
		fmt.Fprintln(d.Stderr, "usage: ff --kanban column new <title>")
		return CommandResult{ExitCode: 1}, nil
	}
}

func (d *CommandDispatcher) kanbanCard(db *DB, args []string) (CommandResult, error) {
	if len(args) < 1 {
		fmt.Fprintln(d.Stderr, "usage: ff --kanban card add <spec_id|title>")
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
	default:
		fmt.Fprintln(d.Stderr, "usage: ff --kanban card add <spec_id|title>")
		return CommandResult{ExitCode: 1}, nil
	}
}

func (d *CommandDispatcher) kanbanList(db *DB) (CommandResult, error) {
	boards, err := db.ListBoards()
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	if len(boards) == 0 {
		fmt.Fprintln(d.Stdout, "No boards. Run 'ff --kanban init' to create one.")
		return CommandResult{ExitCode: 0}, nil
	}
	for _, b := range boards {
		fmt.Fprintf(d.Stdout, "Board: %s (%s)\n", b.Name, b.ID)
		board, err := db.ListBoard(b.ID)
		if err != nil {
			continue
		}
		for _, col := range board.Columns {
			fmt.Fprintf(d.Stdout, "  %s:\n", col.Title)
			for _, card := range col.Cards {
				fmt.Fprintf(d.Stdout, "    - [%s] %s (%s)\n", card.Status, card.Title, card.ID)
			}
		}
	}
	return CommandResult{ExitCode: 0}, nil
}
