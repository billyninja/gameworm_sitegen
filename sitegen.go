package main

import (
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

var re_Article *regexp.Regexp = regexp.MustCompile("\\[\\[.+?\\]\\]")
var re_IsRomanLetter *regexp.Regexp = regexp.MustCompile("[a-zA-Z]{1}")

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

type Index struct {
    Title     string
    Desc      string
    query     string
    pageLimit uint16
}

type TitleIndexInfo struct {
    SourceTitle        string     `db:"source_title"`
    FinalTitle         string     `db:"final_title"`
    WikiPageId         uint64     `db:"wpid"`
    MainInfoboxSubject string     `db:"main_infobox_subject"`
    UpdatedAt          *time.Time `db:"updated_at"`
}

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

type ListingEntry struct {
    InnerRef string
    WikiRef  string
    Title    string
    CRatio   uint8
}

func gen_title_idxs(dbc *sqlx.DB) error {
    println("TODO! ALPHABETIC")
    println("TODO! PAGINATE")
    println("TODO! BY PLATFORM")
    println("TODO! BY AUTHORS")
    println("TODO! BY YEAR")
    return nil
}

func gen_indexes(dbc *sqlx.DB) error {
    q := `SELECT
          source_title,
          final_title,
          ai.wiki_page_id as wpid,
          main_infobox_subject,
          ai.updated_at as updated_at
    FROM article_info ai
    LEFT JOIN assertive_game_info gi
    ON ai.wiki_page_id = gi.wiki_page_id
    ORDER BY final_title;
    `

    m := &Index{
        "All Titles Alphabetic",
        "All Titles from #-to-Z",
        q,
        300,
    }

    var rows *sqlx.Rows

    rows, err := dbc.Queryx(m.query)
    if err != nil {
        log.Printf("Query Err; %+v", err)
        log.Println(m.query)
        return err
    }

    key := "#-9"
    pc := 0
    whole_thing := map[string][][]*TitleIndexInfo{}
    whole_thing[key] = [][]*TitleIndexInfo{}
    for rows.Next() {
        dest := &TitleIndexInfo{}
        err := rows.StructScan(dest)
        if err != nil {
            log.Panicf("\n%+v\n---", err)
        }
        //println(dest.FinalTitle, ic, pc)

        v0 := strings.ToUpper(dest.FinalTitle[0:1])
        is_letter := re_IsRomanLetter.MatchString(v0)
        key = v0
        if !is_letter {
            key = "#-9"
        }

        pc = len(whole_thing[key]) - 1
        if pc < 0 {
            whole_thing[key] = [][]*TitleIndexInfo{}
            whole_thing[key] = append(whole_thing[key], []*TitleIndexInfo{dest})
            pc = 0
        }

        if _, ok := whole_thing[key]; !ok {
            whole_thing[key] = [][]*TitleIndexInfo{}
            pc = 0
            println("here!")
        }

        whole_thing[key][pc] = append(whole_thing[key][pc], dest)
        ic := len(whole_thing[key][pc])
        if uint16(ic) >= m.pageLimit {
            whole_thing[key] = append(whole_thing[key], []*TitleIndexInfo{dest})
        }

        println(key, pc, ic)
    }

    idxtitle := strings.ToLower(strings.Replace(m.Title, " ", "_", -1))
    dirname := fmt.Sprintf("out/idxs/%s", idxtitle)
    err = os.MkdirAll(dirname, 0755)
    if err != nil {
        log.Fatal(err)
    }

    filename := fmt.Sprintf("out/idxs/%s/index.html", idxtitle)
    fh, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0755)
    fh.Close()

    for key, bucket := range whole_thing {
        for pagenum, _ := range bucket {
            println("key", key)
            filename = fmt.Sprintf("%s/%s", dirname, key)
            err := os.MkdirAll(filename, 0755)
            if err != nil {
                log.Fatal(err)
            }
            filename = fmt.Sprintf("%s/page_%d.html", filename, pagenum+1)
            fh, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0755)
            if err != nil {
                log.Fatal(err)
            }
            fh.Close()
        }
    }

    return err
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

        dest.GameModes = re_Article.FindAllString(string(dest.GameModesBytes), -1)
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

    gen_indexes(dbc)
    //gen_title_views(dbc)
}
