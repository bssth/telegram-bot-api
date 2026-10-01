package tgbotapi

import "fmt"

// nestedMediaUploader rewrites InputMedia* values nested inside JSON-serialized
// parameters (poll media, rich messages) so that every file which needs
// uploading is replaced by an "attach://<name>" reference, and collects the
// matching RequestFile entries.
//
// Names are generated sequentially from prefix, so a config's params() and
// files() produce the same names as long as both walk the media in the same
// order (which they do by calling the same prepare function).
type nestedMediaUploader struct {
	prefix string
	files  []RequestFile
}

func (u *nestedMediaUploader) attach(data RequestFileData) RequestFileData {
	if data == nil || !data.NeedsUpload() {
		return data
	}

	name := fmt.Sprintf("%s-%d", u.prefix, len(u.files))
	u.files = append(u.files, RequestFile{Name: name, Data: data})

	return fileAttach("attach://" + name)
}

func (u *nestedMediaUploader) base(b BaseInputMedia) BaseInputMedia {
	b.Media = u.attach(b.Media)
	return b
}

func (u *nestedMediaUploader) photo(m *InputMediaPhoto) *InputMediaPhoto {
	if m == nil {
		return nil
	}
	c := *m
	c.BaseInputMedia = u.base(c.BaseInputMedia)
	return &c
}

func (u *nestedMediaUploader) video(m *InputMediaVideo) *InputMediaVideo {
	if m == nil {
		return nil
	}
	c := *m
	c.BaseInputMedia = u.base(c.BaseInputMedia)
	c.Thumbnail = u.attach(c.Thumbnail)
	c.Cover = u.attach(c.Cover)
	return &c
}

func (u *nestedMediaUploader) animation(m *InputMediaAnimation) *InputMediaAnimation {
	if m == nil {
		return nil
	}
	c := *m
	c.BaseInputMedia = u.base(c.BaseInputMedia)
	c.Thumbnail = u.attach(c.Thumbnail)
	return &c
}

func (u *nestedMediaUploader) audio(m *InputMediaAudio) *InputMediaAudio {
	if m == nil {
		return nil
	}
	c := *m
	c.BaseInputMedia = u.base(c.BaseInputMedia)
	c.Thumbnail = u.attach(c.Thumbnail)
	return &c
}

func (u *nestedMediaUploader) document(m *InputMediaDocument) *InputMediaDocument {
	if m == nil {
		return nil
	}
	c := *m
	c.BaseInputMedia = u.base(c.BaseInputMedia)
	c.Thumbnail = u.attach(c.Thumbnail)
	return &c
}

func (u *nestedMediaUploader) voiceNote(m *InputMediaVoiceNote) *InputMediaVoiceNote {
	if m == nil {
		return nil
	}
	c := *m
	c.BaseInputMedia = u.base(c.BaseInputMedia)
	return &c
}

func (u *nestedMediaUploader) livePhoto(m *InputMediaLivePhoto) *InputMediaLivePhoto {
	if m == nil {
		return nil
	}
	c := *m
	c.BaseInputMedia = u.base(c.BaseInputMedia)
	c.Photo = u.attach(c.Photo)
	return &c
}

func (u *nestedMediaUploader) sticker(m *InputMediaSticker) *InputMediaSticker {
	if m == nil {
		return nil
	}
	c := *m
	c.Media = u.attach(c.Media)
	return &c
}

// media prepares an InputMedia* value or pointer of any kind. Values keep
// their value type and pointers keep their pointer type; anything else (for
// example InputMediaLocation, which carries no file) is returned unchanged.
func (u *nestedMediaUploader) media(m any) any {
	switch v := m.(type) {
	case InputMediaPhoto:
		return *u.photo(&v)
	case *InputMediaPhoto:
		return u.photo(v)
	case InputMediaVideo:
		return *u.video(&v)
	case *InputMediaVideo:
		return u.video(v)
	case InputMediaAnimation:
		return *u.animation(&v)
	case *InputMediaAnimation:
		return u.animation(v)
	case InputMediaAudio:
		return *u.audio(&v)
	case *InputMediaAudio:
		return u.audio(v)
	case InputMediaDocument:
		return *u.document(&v)
	case *InputMediaDocument:
		return u.document(v)
	case InputMediaVoiceNote:
		return *u.voiceNote(&v)
	case *InputMediaVoiceNote:
		return u.voiceNote(v)
	case InputMediaLivePhoto:
		return *u.livePhoto(&v)
	case *InputMediaLivePhoto:
		return u.livePhoto(v)
	case InputMediaSticker:
		return *u.sticker(&v)
	case *InputMediaSticker:
		return u.sticker(v)
	}

	return m
}

func (u *nestedMediaUploader) richBlocks(blocks []InputRichBlock) []InputRichBlock {
	if blocks == nil {
		return nil
	}

	out := make([]InputRichBlock, len(blocks))
	for i, b := range blocks {
		b.Blocks = u.richBlocks(b.Blocks)
		if b.Items != nil {
			items := make([]InputRichBlockListItem, len(b.Items))
			for j, item := range b.Items {
				item.Blocks = u.richBlocks(item.Blocks)
				items[j] = item
			}
			b.Items = items
		}
		b.Animation = u.animation(b.Animation)
		b.Audio = u.audio(b.Audio)
		b.Document = u.document(b.Document)
		b.Photo = u.photo(b.Photo)
		b.Video = u.video(b.Video)
		b.VoiceNote = u.voiceNote(b.VoiceNote)
		out[i] = b
	}

	return out
}

// richMessage returns a copy of m in which every file that needs uploading,
// in Media and anywhere in Blocks, is replaced by an attach:// reference.
func (u *nestedMediaUploader) richMessage(m *InputRichMessage) *InputRichMessage {
	if m == nil {
		return nil
	}

	c := *m
	c.Blocks = u.richBlocks(c.Blocks)
	if c.Media != nil {
		media := make([]InputRichMessageMedia, len(c.Media))
		for i, item := range c.Media {
			item.Media = u.media(item.Media)
			media[i] = item
		}
		c.Media = media
	}

	return &c
}

// prepareRichMessage returns the rich message to serialize and the files
// that must be uploaded alongside it.
func prepareRichMessage(m *InputRichMessage) (*InputRichMessage, []RequestFile) {
	u := nestedMediaUploader{prefix: "rich-media"}
	prepared := u.richMessage(m)
	return prepared, u.files
}
