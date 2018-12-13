package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/VojtechVitek/go-trello"
	trello_search "github.com/adlio/trello"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"github.com/utilitywarehouse/uwgolib/log"
)

const (
	appName = "trello2md"
	appDesc = "exports list from multiple trello boards into a single markdown document"

	dateFormat        = "2006-01-02"
	commentCardAction = "commentCard"
)

var (
	revision string

	globalArguments = []cli.Flag{
		cli.StringFlag{
			Name:   "key",
			Usage:  "trello application key",
			EnvVar: "KEY",
		},
		cli.StringFlag{
			Name:   "token",
			Usage:  "trello api token",
			EnvVar: "TOKEN",
		},
	}

	exportBoardsArguments = []cli.Flag{
		cli.StringSliceFlag{
			Name:   "board-id",
			Usage:  "the trello board ids for boards to export",
			EnvVar: "BOARD_ID",
		},
		cli.StringFlag{
			Name:   "list-filter",
			Usage:  "the filter to apply when looking for lists to export",
			EnvVar: "LIST_FILTER",
			Value:  "Done",
		},
	}

	searchBoardsArgs = []cli.Flag{
		cli.StringFlag{
			Name:   "board-filter",
			Usage:  "the board name filter to apply",
			EnvVar: "BOARD_FILTER",
		},
	}
)

func main() {
	app := cli.NewApp()
	app.Name = appName
	app.Description = appDesc
	app.Version = revision
	app.Flags = globalArguments
	app.Commands = []cli.Command{
		{
			Name:   "export-boards",
			Flags:  exportBoardsArguments,
			Action: exportBoards,
		},
		{
			Name:   "search-boards",
			Flags:  searchBoardsArgs,
			Action: searchBoards,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Panic(err)
	}
}

func searchBoards(c *cli.Context) error {
	client := trello_search.NewClient(c.GlobalString("key"), c.GlobalString("token"))

	boards, err := client.SearchBoards(c.String("board-filter"), trello_search.Defaults())
	if err != nil {
		return err
	}

	for _, board := range boards {
		fmt.Printf("%s - %s\n", board.ID, board.Name)
	}

	return nil
}

func exportBoards(c *cli.Context) error {
	token := c.GlobalString("token")
	client, err := trello.NewAuthClient(c.GlobalString("key"), &token)
	if err != nil {
		return err
	}

	printDate()

	boards, err := getBoards(client, c.StringSlice("board-id"))
	if err != nil {
		return err
	}

	for _, board := range *boards {
		printBoard(board)

		list, err := getList(&board, c.String("list-filter"))
		if err != nil {
			return err
		}

		cards, err := getCards(list)
		if err != nil {
			return err
		}

		for _, card := range *cards {
			err := printCardTitle(&card)
			if err != nil {
				return err
			}

			err = printCardLabelsAndMembers(&card)
			if err != nil {
				return err
			}

			printCardDescription(&card)

			checklists, err := getCardCheckLists(&card)
			if err != nil {
				return err
			}

			for _, checklist := range *checklists {
				printCardChecklist(&checklist)
			}

			commentActions, err := getCardComments(&card)
			if err != nil {
				return err
			}

			for _, commentAction := range *commentActions {
				err := printCardComments(&commentAction)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func printDate() {
	fmt.Printf("## %s\n", time.Now().Format(dateFormat))
}

func getBoards(client *trello.Client, boardIds []string) (*[]trello.Board, error) {
	var boards []trello.Board
	for _, boardId := range boardIds {
		board, err := client.Board(boardId)
		if err != nil {
			return nil, err
		}

		boards = append(boards, *board)
	}

	return &boards, nil
}

func printBoard(board trello.Board) {
	fmt.Printf("### %s\n", board.Name)
}

func getList(board *trello.Board, listFilter string) (*trello.List, error) {
	lists, err := board.Lists()
	if err != nil {
		return nil, err
	}

	for _, list := range lists {
		if list.Name == listFilter {
			return &list, nil
		}
	}

	return nil, errors.New("no matching list found")
}

func getCards(list *trello.List) (*[]trello.Card, error) {
	cards, err := list.Cards()
	if err != nil {
		return nil, err
	}

	return &cards, nil
}

func printCardTitle(card *trello.Card) error {
	lastActivity, err := time.Parse(time.RFC3339, card.DateLastActivity)
	if err != nil {
		return err
	}

	fmt.Printf("#### **%s** [%s](%s)\n", lastActivity.Format(dateFormat), card.Name, card.Url)

	return nil
}

func printCardLabelsAndMembers(card *trello.Card) error {
	fmt.Printf("##### ")
	for _, label := range card.Labels {
		fmt.Printf("`%s` ", label.Name)
	}

	members, err := card.Members()
	if err != nil {
		return err
	}

	var memberNames []string
	for _, member := range members {
		memberNames = append(memberNames, member.FullName)
	}

	fmt.Printf("- **[%s]**\n", strings.Join(memberNames, ", "))

	return nil
}

func printCardDescription(card *trello.Card) {
	fmt.Printf("%s\n\n", card.Desc)
}

func getCardCheckLists(card *trello.Card) (*[]trello.Checklist, error) {
	checklists, err := card.Checklists()
	if err != nil {
		return nil, err
	}

	return &checklists, nil
}

func printCardChecklist(checklist *trello.Checklist) {
	fmt.Printf("**%s**\n", checklist.Name)
	for _, checkItem := range checklist.CheckItems {
		if checkItem.State == "complete" {
			fmt.Printf("- [x] %s\n", checkItem.Name)
		} else {
			fmt.Printf("- [ ] %s\n", checkItem.Name)
		}
	}
	fmt.Printf("\n")
}

func getCardComments(card *trello.Card) (*[]trello.Action, error) {
	actions, err := card.Actions()
	if err != nil {
		return nil, err
	}

	var commentCardActions []trello.Action
	for _, action := range actions {
		if action.Type == commentCardAction {
			commentCardActions = append(commentCardActions, action)
		}
	}

	return &commentCardActions, nil
}

func printCardComments(commentAction *trello.Action) error {
	actionDate, err := time.Parse(time.RFC3339, commentAction.Date)
	if err != nil {
		return err
	}

	fmt.Printf("> **%s** - **%s:**\n> \n", actionDate.Format(dateFormat), commentAction.MemberCreator.FullName)
	fmt.Printf("> %s\n\n", strings.Replace(commentAction.Data.Text, "\n", "\n> ", -1))

	return nil
}
