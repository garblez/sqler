package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "github.com/go-sql-driver/mysql"
)

const listHeight = 14

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("170"))
	paginationStyle = list.DefaultStyles().
			PaginationStyle.
			PaddingLeft(4)
	helpStyle = list.DefaultStyles().
			HelpStyle.
			PaddingLeft(4).
			PaddingBottom(1)
	quitTextStyle = lipgloss.NewStyle().
			Margin(1, 0, 2, 4)
)

type item string

func (i item) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

type model struct {
	list     list.Model
	choice   string
	quitting bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = string(i)
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.choice != "" {
		return quitTextStyle.Render(fmt.Sprintf("Let's take a look at %s then...", m.choice))
	}

	if m.quitting {
		return quitTextStyle.Render("Goodbye...")
	}
	return "\n" + m.list.View()
}

// DB stuff below:

var user, password, host, database string
var port int

func init() {
	const (
		defaultUser     = "root"
		defaultPassword = ""
		defaultHost     = "localhost"
		defaultDB       = ""
		defaultPort     = 3306
	)
	flag.StringVar(&user, "user", defaultUser, "database administrator account for sign-in")
	flag.StringVar(&password, "password", defaultPassword, "password for the administrator account")
	flag.StringVar(&host, "host", defaultHost, "hostname for server on which database is hosted")
	flag.StringVar(&database, "database", defaultDB, "the database to access")
	flag.IntVar(&port, "port", defaultPort, "the port the database server is on")

}

func dbURI() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", user, password, host, port, database)
}

func dbTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}

	tables := make([]string, 0)

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tables, nil

}

func allTableRows(db *sql.DB, table string) ([]byte, error) {
	query := fmt.Sprintf("SELECT * FROM %s.%s", database, table)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	colLen := len(cols)
	// Retrieve a list of name->value maps like in JSON
	namedColumns := make([]map[string]interface{}, 0)
	// Pass interfaces to sql.Row.Scan
	colVals := make([]interface{}, colLen)

	for rows.Next() {
		colAssoc := make(map[string]interface{}, len(cols))
		for i := range colVals {
			colVals[i] = new(interface{})
		}
		if err := rows.Scan(colVals...); err != nil {
			return nil, err
		}
		for i, col := range cols {
			grabbedValue := *colVals[i].(*interface{})
			if fmt.Sprintf("%T", grabbedValue) == "[]uint8" {
				colAssoc[col] = fmt.Sprintf("%s", grabbedValue)
			} else {
				colAssoc[col] = grabbedValue
			}
		}
		namedColumns = append(namedColumns, colAssoc)
	}

	json, err := json.Marshal(namedColumns)
	if err != nil {
		return nil, err
	}

	return json, nil
}

func main() {
	flag.Parse()
	uri := dbURI()
	db, err := sql.Open("mysql", uri)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	tables, err := dbTables(db)
	if err != nil {
		panic(err.Error())
	}

	items := make([]list.Item, 0)
	for _, t := range tables {
		items = append(items, item(t))
	}

	const defaultWidth = 20

	jsonResults, err := allTableRows(db, "ProgrammingLanguages")
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(jsonResults))

	jsonResults, err = allTableRows(db, "Notes")
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(jsonResults))

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = fmt.Sprintf("Tables in the %s database:", database)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	m := model{list: l}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
