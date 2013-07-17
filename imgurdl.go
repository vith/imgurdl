package main

import (
  "flag"
  "fmt"
  "io"
  "io/ioutil"
  "log"
  "net/http"
  "os"
  "regexp"
  //"path/filepath" TODO: use this
)

var client *http.Client

func init() {
  flag.Parse()
  client = new(http.Client)
}

func main() {
  for _, arg := range flag.Args() {

    target := parseTarget(arg)
    resp := doGet(target)

    switch target.targetType {
    case "image":
      saveImage(target, resp)
    case "album":
      saveAlbum(target, resp)
    }
  }
}

type imgurTarget struct {
  url        string
  hash       string
  targetType string
  outDir     string // requires trailing slash, e.g. "subdir/"

}

var albumUrlRe = regexp.MustCompile(`^(?:(?:https?://)?imgur.com/a/)?([A-Za-z0-9]{5})(?:$|#)`)
var imageUrlRe = regexp.MustCompile(`^(?:https?://)?(?:(?:i\.)?imgur\.com/)?([A-Za-z0-9]{7})`)

func parseTarget(input string) (target imgurTarget) {
  if albumHash := albumUrlRe.FindStringSubmatch(input); albumHash != nil {
    target.targetType = "album"
    target.hash = albumHash[1]
    target.url = fmt.Sprintf("http://imgur.com/a/%s", target.hash)
    target.outDir = target.hash
    return target
  } else if imageHash := imageUrlRe.FindStringSubmatch(input); imageHash != nil {
    target.targetType = "image"
    target.hash = imageHash[1]
    target.url = fmt.Sprintf("http://i.imgur.com/%s.jpg", target.hash)
    target.outDir = ""
    return target
  }
  panic(fmt.Sprintf("unknown target format: %s", input))
  return target
}

func doGet(target imgurTarget) (resp *http.Response) {
  //prepare Request
  req, _ := http.NewRequest("GET", target.url, nil)
  req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:19.0) Gecko/20100101 Firefox/19.0")

  //send GET
  resp, err := client.Do(req)
  if err != nil {
    log.Fatal(err)
  }

  return resp
}

var albumJsRe = regexp.MustCompile(`(?s)<script type="text/javascript">\s*var album = Imgur\.Album\.getInstance\((.*?)\);\s*</script>`)
var albumJsHashesRe = regexp.MustCompile(`"hash"\s*:\s*"(\w{7}|\w{5})"`)

func processPage(body []byte) (hashes []string) {
  if album := albumJsRe.Find(body); album != nil {
    log.Println("detected album")
    //fmt.Printf("album: %s\n", album)
    hashMatches := albumJsHashesRe.FindAllSubmatch(album, -1)
    hashes := make([]string, len(hashMatches))
    for k, v := range hashMatches {
      hashes[k] = string(v[1])
    }
    fmt.Printf("hashes: %s\n", hashes)
    return hashes
  }
  panic("could not find albumJs")
  return nil
}

func saveImage(target imgurTarget, resp *http.Response) (bytesSaved int64, err error) {
  var fileExt, contentType string

  contentType = resp.Header.Get("Content-Type")
  switch contentType {
  case "image/jpeg":
    fileExt = "jpg"
  default:
    panic(fmt.Sprintf("unexpected Content-Type: %s for url: %s", contentType, target.url))
  }

  filename := fmt.Sprintf("%s%s.%s", target.outDir, target.hash, fileExt)
  fmt.Printf("saving %s\n", filename)
  file, err := os.Create(filename)
  defer file.Close()
  if err != nil {
    panic(err)
  }
  bytesSaved, err = io.Copy(file, resp.Body)
  resp.Body.Close()
  if err != nil {
    panic(err)
  }
  return bytesSaved, err
}

func saveAlbum(target imgurTarget, resp *http.Response) (imagesSaved int, err error) {
  contentType := resp.Header.Get("Content-Type")
  switch contentType {
  case "text/html; charset=utf-8":
    body, _ := ioutil.ReadAll(resp.Body)
    resp.Body.Close()
    albumHashes := processPage(body)
    os.Mkdir(target.hash, 0777)
    for _, albumMember := range albumHashes {
      memberTarget := parseTarget(albumMember)
      memberTarget.outDir = fmt.Sprintf("%s/", target.hash)
      memberResp := doGet(memberTarget)
      saveImage(memberTarget, memberResp)
    }
  default:
    panic(fmt.Sprintf("unexpected Content-Type: %s for url: %s", contentType, target.url))
  }
  return 0, nil
}
