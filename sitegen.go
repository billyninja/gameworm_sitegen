package main

import (
    // "flag"
    //    "encoding/json"
    "fmt"
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
    "html/template"
    "log"
    "os"
    "regexp"
    "strings"
    "time"
)

var re *regexp.Regexp = regexp.MustCompile("\\[\\[.+?\\]\\]")

func ping(dbc *sqlx.DB) {
    pingq := "SELECT COUNT(*) FROM article_info"

    var rows *sqlx.Rows
    rows, err := dbc.Queryx(pingq)
    if err != nil {
        log.Println("%v", err)
        log.Println(pingq)
    }

    log.Println("Pinged!", rows)
}

type TitleView struct {
    Idx  int
    Blob string
}

func gen_title_view(tpl *template.Template, idx int, tinfo *TitleViewInfo) error {
    filename := fmt.Sprintf("out/%s.html", strings.Replace(tinfo.FinalTitle, "/", "_", -1))
    fh, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0755)
    if err != nil {
        log.Fatal(err)
    }
    defer fh.Close()

    tpl.ExecuteTemplate(fh, "title_view_admin.html", tinfo)

    return err
}

type GameMode string

type TitleViewInfo struct {
    SourceTitle        string     `db:"source_title"`
    FinalTitle         string     `db:"final_title"`
    WikiPageId         uint64     `db:"wpid"`
    MainInfoboxSubject string     `db:"main_infobox_subject"`
    UpdatedAt          *time.Time `db:"updated_at"`
    WikimediaImage     string     `db:"wikimedia_image"`
    ImageCaption       string     `db:"image_caption"`
    GameModesBytes     []uint8    `db:"game_modes"`
    AuthorsBytes       []uint8    `db:"authors"`
    CompaniesBytes     []uint8    `db:"companies"`
    EnginesBytes       []uint8    `db:"engines"`
    IAuthors           []*AuthorInfo
    ICompanies         []*AuthorInfo
    IEngines           []string
    GameModes          []string
}

type AuthorInfo struct {
    Name string `db:"source_title"`
    Role string `db:"final_title"`
}

func gen_title_views(dbc *sqlx.DB) error {
    tp1, err := template.ParseFiles("templates/title_view_admin.html")
    if err != nil {
        log.Panicf("%v", err)
    }
    log.Println("template loaded!", tp1)

    fetchQ := `
    SELECT
          source_title,
          final_title,
          ai.wiki_page_id as wpid,
          main_infobox_subject,
          ai.updated_at as updated_at,
          COALESCE(wikimedia_image, '') as wikimedia_image,
          COALESCE(image_caption, '') as image_caption,
          game_modes,
          (SELECT array_agg(CONCAT(ga.author_role, ' : ', ga.name)) from game_info_author ga WHERE ga.game_wpi = ai.wiki_page_id)
          as authors,
          (SELECT array_agg(ge.name) from game_info_engine ge WHERE ge.game_wpi = ai.wiki_page_id)
          as engines,
          (SELECT array_agg(CONCAT(gp.company_role, ' : ', gp.company_name)) from game_info_company gp WHERE gp.game_wpi = ai.wiki_page_id)
          as companies
          -- (SELECT array_agg(json_build_object('code', gr.region_code, 'slug', gr.platform_slug, 'date', gr.release_date)) from game_info_release gr WHERE gr.game_wpi = ai.wiki_page_id)
          -- as releases
    FROM article_info ai
    LEFT JOIN assertive_game_info gi
    ON ai.wiki_page_id = gi.wiki_page_id
    ORDER BY final_title;`

    var rows *sqlx.Rows

    rows, err = dbc.Queryx(fetchQ)
    if err != nil {
        log.Printf("Query Err; %+v", err)
        log.Println(fetchQ)
        return err
    }

    i := 1
    for rows.Next() {
        dest := &TitleViewInfo{}
        err := rows.StructScan(dest)
        if err != nil {
            log.Panicf("\n%+v\n---", err)
        }

        dest.GameModes = re.FindAllString(string(dest.GameModesBytes), -1)
        if len(dest.AuthorsBytes) > 0 {
            xx := dest.AuthorsBytes[2 : len(dest.AuthorsBytes)-2]
            spl1 := strings.Split(string(xx), `","`)
            for _, author := range spl1 {
                spl2 := strings.Split(author, ` : `)
                dest.IAuthors = append(dest.IAuthors, &AuthorInfo{spl2[1], spl2[0]})
            }
        }

        if len(dest.CompaniesBytes) > 0 {
            xx := dest.CompaniesBytes[2 : len(dest.CompaniesBytes)-2]
            spl1 := strings.Split(string(xx), `","`)
            for _, author := range spl1 {
                spl2 := strings.Split(author, ` : `)
                dest.ICompanies = append(dest.ICompanies, &AuthorInfo{spl2[1], spl2[0]})
            }
        }

        if len(dest.EnginesBytes) > 0 {
            xx := dest.EnginesBytes[2 : len(dest.EnginesBytes)-2]
            println(string(xx))
            dest.IEngines = strings.Split(string(xx), `","`)
        }

        gen_title_view(tp1, i, dest)
        i++
    }

    log.Printf("\n\nGenerated %d title view pages!\n\n", i-1)

    return err
}

func main() {
    log.Println("Initializing...")

    db := "gameworm_wikipedia"
    user := "postgres"
    pass := "postgres"
    host := "localhost"
    port := "5432"

    strConn := fmt.Sprintf(
        "dbname=%s user=%s password=%s host=%s port=%s sslmode=disable",
        db, user, pass, host, port,
    )
    dbc := sqlx.MustOpen("postgres", strConn)
    ping(dbc)
    log.Println("Connected!", dbc)

    gen_title_views(dbc)
}
