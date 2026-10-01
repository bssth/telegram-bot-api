package tgbotapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOwnedGiftGiftRouting(t *testing.T) {
	var regular OwnedGift
	if err := json.Unmarshal([]byte(`{"type":"regular","gift":{"id":"g1","star_count":10},"send_date":1}`), &regular); err != nil {
		t.Fatalf("regular unmarshal: %v", err)
	}
	if regular.Gift == nil || regular.Gift.ID != "g1" || regular.UniqueGift != nil {
		t.Fatalf("regular gift not routed to Gift: %+v", regular)
	}

	var unique OwnedGift
	if err := json.Unmarshal([]byte(`{"type":"unique","gift":{"base_name":"Cap","name":"Cap-1","number":1},"send_date":1}`), &unique); err != nil {
		t.Fatalf("unique unmarshal: %v", err)
	}
	if unique.UniqueGift == nil || unique.UniqueGift.Name != "Cap-1" || unique.Gift != nil {
		t.Fatalf("unique gift not routed to UniqueGift: %+v", unique)
	}

	out, err := json.Marshal(unique)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"gift":{`) || strings.Contains(string(out), "unique_gift") {
		t.Fatalf("unique gift not emitted under \"gift\": %s", out)
	}
}

func TestSpecTypesDecode(t *testing.T) {
	var video Video
	if err := json.Unmarshal([]byte(`{"file_id":"v","file_unique_id":"u","width":1,"height":1,"duration":1,
		"qualities":[{"file_id":"q","file_unique_id":"qu","width":1920,"height":1080,"codec":"av01","file_size":42}]}`), &video); err != nil {
		t.Fatalf("video unmarshal: %v", err)
	}
	if len(video.Qualities) != 1 || video.Qualities[0].Codec != "av01" || video.Qualities[0].Height != 1080 || video.Qualities[0].FileSize != 42 {
		t.Fatalf("video qualities not decoded: %+v", video.Qualities)
	}

	var chat ChatFullInfo
	if err := json.Unmarshal([]byte(`{"id":1,"type":"private","bio":"hi","accent_color_id":3,"max_reaction_count":11,
		"accepted_gift_types":{},"rating":{"level":2,"rating":150,"current_level_rating":100,"next_level_rating":200},
		"unique_gift_colors":{"model_custom_emoji_id":"m","symbol_custom_emoji_id":"s","light_theme_main_color":1,
		"light_theme_other_colors":[2,3],"dark_theme_main_color":4,"dark_theme_other_colors":[5]}}`), &chat); err != nil {
		t.Fatalf("chat unmarshal: %v", err)
	}
	if chat.ID != 1 || chat.Bio != "hi" || chat.AccentColorID != 3 {
		t.Fatalf("chat full info fields not decoded: %+v", chat)
	}
	if chat.Rating == nil || chat.Rating.Level != 2 || chat.Rating.NextLevelRating != 200 {
		t.Fatalf("user rating not decoded: %+v", chat.Rating)
	}
	if chat.UniqueGiftColors == nil || len(chat.UniqueGiftColors.LightThemeOtherColors) != 2 {
		t.Fatalf("unique gift colors not decoded: %+v", chat.UniqueGiftColors)
	}

	var msg Message
	if err := json.Unmarshal([]byte(`{"message_id":1,"date":1,"chat":{"id":1,"type":"group"},
		"poll_option_added":{"option_persistent_id":"p","option_text":"new","option_text_entities":[{"type":"bold","offset":0,"length":3}]},
		"chat_owner_left":{"new_owner":{"id":7,"is_bot":false,"first_name":"N"}},
		"entities":[{"type":"date_time","offset":0,"length":1,"unix_time":1700000000,"date_time_format":"wDT"}]}`), &msg); err != nil {
		t.Fatalf("message unmarshal: %v", err)
	}
	if msg.PollOptionAdded == nil || msg.PollOptionAdded.OptionText != "new" || len(msg.PollOptionAdded.OptionTextEntities) != 1 {
		t.Fatalf("poll option added not decoded: %+v", msg.PollOptionAdded)
	}
	if msg.ChatOwnerLeft == nil || msg.ChatOwnerLeft.NewOwner == nil || msg.ChatOwnerLeft.NewOwner.ID != 7 {
		t.Fatalf("chat owner left not decoded: %+v", msg.ChatOwnerLeft)
	}
	if len(msg.Entities) != 1 || msg.Entities[0].UnixTime != 1700000000 || msg.Entities[0].DateTimeFormat != "wDT" {
		t.Fatalf("date_time entity not decoded: %+v", msg.Entities)
	}
}

func TestUpdateFromChatInlineCallback(t *testing.T) {
	// Callback queries from inline messages carry no message; FromChat must
	// not panic.
	update := Update{CallbackQuery: &CallbackQuery{ID: "1", InlineMessageID: "inline"}}
	if chat := update.FromChat(); chat != nil {
		t.Fatalf("FromChat() = %+v, want nil", chat)
	}

	update = Update{ChatJoinRequest: &ChatJoinRequest{Chat: Chat{ID: 5}, From: User{ID: 6}}}
	if chat := update.FromChat(); chat == nil || chat.ID != 5 {
		t.Fatalf("FromChat() for chat join request = %+v, want chat 5", chat)
	}
	if from := update.SentFrom(); from == nil || from.ID != 6 {
		t.Fatalf("SentFrom() for chat join request = %+v, want user 6", from)
	}
}

func paramsOf(t *testing.T, c Chattable) Params {
	t.Helper()
	params, err := c.params()
	if err != nil {
		t.Fatalf("%T.params(): %v", c, err)
	}
	return params
}

func TestSpecMethodParams(t *testing.T) {
	params := paramsOf(t, RepostStoryConfig{BusinessConnectionID: "bc", FromChatID: 1, FromStoryID: 2, ActivePeriod: 86400})
	for key, want := range map[string]string{"business_connection_id": "bc", "from_chat_id": "1", "from_story_id": "2", "active_period": "86400"} {
		if params[key] != want {
			t.Errorf("repostStory %s = %q, want %q", key, params[key], want)
		}
	}

	params = paramsOf(t, GetChatGiftsConfig{ChatID: 1, ExcludeUnsaved: true, SortByPrice: true, ExcludeFromBlockchain: true})
	for _, key := range []string{"exclude_unsaved", "sort_by_price", "exclude_from_blockchain"} {
		if params[key] != "true" {
			t.Errorf("getChatGifts %s = %q, want true", key, params[key])
		}
	}

	params = paramsOf(t, SetGameScoreConfig{UserID: 1, Score: 0, Force: true, ChatID: 2, MessageID: 3})
	if params["score"] != "0" || params["force"] != "true" {
		t.Errorf("setGameScore params = %v, want score=0 and force=true", params)
	}

	params = paramsOf(t, SetStickerSetThumbnailConfig{Name: "set", UserID: 1, Format: StickerFormatStatic})
	if params["format"] != StickerFormatStatic {
		t.Errorf("setStickerSetThumbnail format = %q", params["format"])
	}
	if files := (SetStickerSetThumbnailConfig{Name: "set"}).files(); len(files) != 0 {
		t.Errorf("setStickerSetThumbnail without thumbnail has files: %v", files)
	}

	params = paramsOf(t, DocumentConfig{
		BaseFile:        BaseFile{BaseChat: BaseChat{ChatID: 1}},
		CaptionEntities: []MessageEntity{{Type: "bold", Offset: 0, Length: 1}},
	})
	if !strings.Contains(params["caption_entities"], `"bold"`) {
		t.Errorf("sendDocument caption_entities = %q", params["caption_entities"])
	}

	params = paramsOf(t, VideoConfig{BaseFile: BaseFile{BaseChat: BaseChat{ChatID: 1}}, Width: 640, Height: 480})
	if params["width"] != "640" || params["height"] != "480" {
		t.Errorf("sendVideo width/height = %q/%q", params["width"], params["height"])
	}

	params = paramsOf(t, MessageConfig{
		BaseChat: BaseChat{ChatID: 1, SuggestedPostParameters: &SuggestedPostParameters{SendDate: 100}},
		Text:     "hi",
	})
	if !strings.Contains(params["suggested_post_parameters"], `"send_date":100`) {
		t.Errorf("sendMessage suggested_post_parameters = %q", params["suggested_post_parameters"])
	}

	params = paramsOf(t, SetPassportDataErrorsConfig{UserID: 1, Errors: []PassportElementError{
		PassportElementErrorUnspecified{Source: "unspecified", Type: "passport", ElementHash: "h", Message: "m"},
	}})
	if !strings.Contains(params["errors"], `"element_hash":"h"`) {
		t.Errorf("setPassportDataErrors errors = %q", params["errors"])
	}
	if (SetPassportDataErrorsConfig{}).method() != "setPassportDataErrors" {
		t.Error("SetPassportDataErrorsConfig has the wrong method")
	}
	if (ChatMemberCountConfig{}).method() != "getChatMemberCount" {
		t.Error("ChatMemberCountConfig uses the deprecated method name")
	}
}

func TestSendPollMediaUpload(t *testing.T) {
	config := SendPollConfig{
		BaseChat: BaseChat{ChatID: 1},
		Question: "q",
		Media:    NewInputMediaPhoto(FileBytes{Name: "q.jpg", Bytes: []byte("q")}),
		Options: []InputPollOption{
			{Text: "a", Media: NewInputMediaPhoto(FileID("existing"))},
			{Text: "b", Media: &InputMediaSticker{Type: "sticker", Media: FilePath("tests/image.jpg")}},
		},
		ExplanationMedia: InputMediaLocation{Type: "location", Latitude: 1, Longitude: 2},
	}

	params := paramsOf(t, config)
	if !strings.Contains(params["media"], `"media":"attach://poll-media-0"`) {
		t.Errorf("media = %s", params["media"])
	}
	if !strings.Contains(params["options"], `"media":"existing"`) || !strings.Contains(params["options"], `"media":"attach://poll-media-1"`) {
		t.Errorf("options = %s", params["options"])
	}
	if !strings.Contains(params["explanation_media"], `"latitude":1`) {
		t.Errorf("explanation_media = %s", params["explanation_media"])
	}

	files := config.files()
	if len(files) != 2 || files[0].Name != "poll-media-0" || files[1].Name != "poll-media-1" {
		t.Fatalf("files = %+v", files)
	}

	// The caller's config must not be modified.
	if _, ok := config.Media.(InputMediaPhoto).Media.(FileBytes); !ok {
		t.Errorf("SendPollConfig.Media was modified: %+v", config.Media)
	}
}

func TestRichMessageMediaUpload(t *testing.T) {
	photo := NewInputMediaPhoto(FilePath("tests/image.jpg"))
	config := SendRichMessageConfig{
		BaseChat: BaseChat{ChatID: 1},
		RichMessage: &InputRichMessage{
			Markdown: "![](tg://photo?id=p1)",
			Media:    []InputRichMessageMedia{{ID: "p1", Media: NewInputMediaPhoto(FileBytes{Name: "a.jpg", Bytes: []byte("a")})}},
			Blocks: []InputRichBlock{{
				Type: "list",
				Items: []InputRichBlockListItem{{Blocks: []InputRichBlock{{
					Type:  "photo",
					Photo: &photo,
				}}}},
			}},
		},
	}

	params := paramsOf(t, config)
	if !strings.Contains(params["rich_message"], `"attach://rich-media-0"`) || !strings.Contains(params["rich_message"], `"attach://rich-media-1"`) {
		t.Errorf("rich_message = %s", params["rich_message"])
	}

	files := config.files()
	if len(files) != 2 {
		t.Fatalf("files = %+v", files)
	}
	if _, ok := photo.Media.(FilePath); !ok {
		t.Errorf("nested block photo was modified: %+v", photo)
	}

	edit := EditMessageTextConfig{BaseEdit: BaseEdit{ChatID: 1, MessageID: 2}, RichMessage: config.RichMessage}
	params = paramsOf(t, edit)
	if _, ok := params["text"]; ok {
		t.Errorf("editMessageText sends an empty text alongside rich_message: %v", params)
	}
	if len(edit.files()) != 2 {
		t.Errorf("editMessageText files = %+v", edit.files())
	}
}
