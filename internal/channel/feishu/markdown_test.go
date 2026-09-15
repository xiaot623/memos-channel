package feishu

import "testing"

func TestPostToMarkdown(t *testing.T) {
	raw := `{
		"title": "Hello",
		"content": [[
			{"tag":"text","text":"bold","style":["bold"]},
			{"tag":"text","text":" and "},
			{"tag":"a","href":"https://example.com","text":"link"},
			{"tag":"text","text":" "},
			{"tag":"text","text":"it","style":["italic"]}
		]]
	}`
	got, images := postToMarkdown(raw)
	want := "Hello\n**bold** and [link](https://example.com) *it*"
	if got != want {
		t.Fatalf("markdown:\nwant %q\ngot  %q", want, got)
	}
	if len(images) != 0 {
		t.Fatalf("unexpected images %v", images)
	}
}

func TestPostToMarkdownImagesAndLocale(t *testing.T) {
	raw := `{
		"zh_cn": {
			"title": "",
			"content": [[
				{"tag":"text","text":"pic"},
				{"tag":"img","image_key":"img_1"}
			]]
		}
	}`
	got, images := postToMarkdown(raw)
	if got != "pic" {
		t.Fatalf("got %q", got)
	}
	if len(images) != 1 || images[0] != "img_1" {
		t.Fatalf("images %v", images)
	}
}
