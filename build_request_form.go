package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"reflect"
	"strings"

	"github.com/go-telegram/bot/models"
)

var (
	customMarshalInterface = reflect.TypeFor[customMarshal]()

	inputMediaInterface = reflect.TypeFor[inputMedia]()
)

type (
	inputMedia interface {
		MarshalInputMedia() ([]byte, error)
		Attachment() io.Reader
		GetMedia() string
	}

	customMarshal interface {
		MarshalCustom() ([]byte, error)
	}

	mediaThumbnail interface {
		GetThumbnail() models.InputFile
	}

	formWriter interface {
		CreateFormField(fieldName string) (io.Writer, error)
		CreateFormFile(fieldName, filename string) (io.Writer, error)
	}

	requestForm struct {
		w          formWriter
		fileParts  map[string]io.Reader
		fieldNames map[string]struct{}
	}
)

func newRequestForm(w formWriter) *requestForm {
	return &requestForm{
		w:          w,
		fileParts:  map[string]io.Reader{},
		fieldNames: map[string]struct{}{},
	}
}

func (f *requestForm) hasFilePart(partName string) bool {
	_, ok := f.fileParts[partName]
	return ok
}

func (f *requestForm) addFilePart(partName, filename string, data io.Reader) error {
	if written, ok := f.fileParts[partName]; ok {
		if readerIsNil(data) || sameReader(written, data) {
			return nil
		}

		return fmt.Errorf("duplicate form part name %q", partName)
	}

	if _, ok := f.fieldNames[partName]; ok {
		return fmt.Errorf("duplicate form part name %q", partName)
	}

	w, errCreatePart := f.w.CreateFormFile(partName, filename)
	if errCreatePart != nil {
		return errCreatePart
	}

	f.fileParts[partName] = data
	_, errCopy := io.Copy(w, data)
	return errCopy
}

func (f *requestForm) addFieldPart(fieldName string, value io.Reader) error {
	if _, ok := f.fieldNames[fieldName]; ok {
		return fmt.Errorf("duplicate form part name %q", fieldName)
	}

	if _, ok := f.fileParts[fieldName]; ok {
		return fmt.Errorf("duplicate form part name %q", fieldName)
	}

	w, errCreateField := f.w.CreateFormField(fieldName)
	if errCreateField != nil {
		return errCreateField
	}

	f.fieldNames[fieldName] = struct{}{}
	_, errCopy := io.Copy(w, value)
	return errCopy
}

func buildRequestForm(w *multipart.Writer, params any) (int, error) {
	form := acquireRequestForm(w)
	defer releaseRequestForm(form)

	v := reflect.ValueOf(params).Elem()

	var fieldsCount int

	for i := range v.NumField() {
		jsonTag := v.Type().Field(i).Tag.Get("json")
		if jsonTag == "-" || jsonTag == "" {
			continue
		}

		fieldName, _, _ := strings.Cut(jsonTag, ",")
		omitempty := strings.Contains(jsonTag, ",omitempty")

		if omitempty && v.Field(i).IsZero() {
			continue
		}

		if v.Field(i).Type().Implements(customMarshalInterface) {
			err := addFormFieldCustomMarshal(form, fieldName, v.Field(i).Interface().(customMarshal))
			if err != nil {
				return 0, err
			}

			fieldsCount++
			continue
		}

		if v.Field(i).Type().Implements(inputMediaInterface) {
			err := addFormFieldInputMedia(form, fieldName, v.Field(i).Interface().(inputMedia))
			if err != nil {
				return 0, err
			}

			fieldsCount++
			continue
		}

		if v.Field(i).Kind() == reflect.Interface && !v.Field(i).IsNil() {
			if v.Field(i).Elem().Type().Implements(customMarshalInterface) {
				err := addFormFieldCustomMarshal(form, fieldName, v.Field(i).Interface().(customMarshal))
				if err != nil {
					return 0, err
				}

				fieldsCount++
				continue
			}

			if v.Field(i).Elem().Type().Implements(inputMediaInterface) {
				err := addFormFieldInputMedia(form, fieldName, v.Field(i).Interface().(inputMedia))
				if err != nil {
					return 0, err
				}

				fieldsCount++
				continue
			}
		}

		var err error

		switch vv := v.Field(i).Interface().(type) {
		case string:
			err = addFormFieldString(form, fieldName, vv)
		case *models.InputFileUpload:
			err = addFormFieldInputFileUpload(form, fieldName, vv)
		case *models.InputFileString:
			if vv == nil {
				return 0, fmt.Errorf("nil value for field %s", fieldName)
			}

			err = addFormFieldString(form, fieldName, vv.Data)
		case *models.InputMediaLivePhoto:
			err = addFormFieldInputMedia(form, fieldName, vv)
		case []models.InputMedia:
			ss := make([]inputMedia, len(vv))
			for i, m := range vv {
				ss[i] = m
			}

			err = addFormFieldInputMediaSlice(form, fieldName, ss)
		case []models.InputPaidMedia:
			ss := make([]inputMedia, len(vv))
			for i, m := range vv {
				ss[i] = m
			}

			err = addFormFieldInputMediaSlice(form, fieldName, ss)
		case []models.InlineQueryResult:
			err = addFormFieldInlineQueryResultSlice(form, fieldName, vv)
		case []models.InputSticker:
			err = addFormFieldInputStickerSlice(form, fieldName, vv)
		case models.InputRichMessage:
			err = addFormFieldInputRichMessage(form, fieldName, &vv)
		case *models.InputRichMessage:
			err = addFormFieldInputRichMessage(form, fieldName, vv)
		default:
			err = addFormFieldDefault(form, fieldName, v.Field(i).Interface())
		}

		if err != nil {
			return 0, err
		}

		fieldsCount++
	}

	return fieldsCount, nil
}

func readerIsNil(r io.Reader) bool {
	return isNilValue(r)
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return v.IsNil()
	default:
		return false
	}
}

func sameReader(a, b io.Reader) bool {
	ta := reflect.TypeOf(a)
	if ta == nil || ta != reflect.TypeOf(b) || !ta.Comparable() {
		return false
	}

	return a == b
}

func addFormFieldInputFileUpload(form *requestForm, fieldName string, value *models.InputFileUpload) error {
	if isNilValue(value) {
		return fmt.Errorf("nil value for field %s", fieldName)
	}

	if readerIsNil(value.Data) {
		return fmt.Errorf("nil data for field %s", fieldName)
	}

	return form.addFilePart(fieldName, value.Filename, value.Data)
}

func addFormFieldInputMediaItem(form *requestForm, value inputMedia) ([]byte, error) {
	if err := addInputMediaAttachment(form, value); err != nil {
		return nil, err
	}

	return value.MarshalInputMedia()
}

func addInputMediaAttachment(form *requestForm, value inputMedia) error {
	if isNilValue(value) {
		return fmt.Errorf("nil input media")
	}

	if filename, ok := strings.CutPrefix(value.GetMedia(), "attach://"); ok {
		if !form.hasFilePart(filename) && readerIsNil(value.Attachment()) {
			return fmt.Errorf("nil attachment for attach://%s", filename)
		}

		if err := form.addFilePart(filename, filename, value.Attachment()); err != nil {
			return err
		}
	}

	if thumbnailed, ok := value.(mediaThumbnail); ok {
		if err := addInputFileAttachment(form, thumbnailed.GetThumbnail()); err != nil {
			return err
		}
	}

	if live, ok := value.(*models.InputMediaLivePhoto); ok && strings.HasPrefix(live.Photo, "attach://") {
		filename := strings.TrimPrefix(live.Photo, "attach://")
		if !form.hasFilePart(filename) && readerIsNil(live.PhotoAttachment) {
			return fmt.Errorf("nil PhotoAttachment for attach://%s", filename)
		}

		if err := form.addFilePart(filename, filename, live.PhotoAttachment); err != nil {
			return err
		}
	}

	return nil
}

func addInputFileAttachment(form *requestForm, value models.InputFile) error {
	upload, ok := value.(*models.InputFileUpload)
	if !ok || upload == nil {
		return nil
	}

	if upload.Filename == "" {
		return fmt.Errorf("empty filename for nested upload, it is the attach:// reference")
	}

	if !form.hasFilePart(upload.Filename) && readerIsNil(upload.Data) {
		return fmt.Errorf("nil data for attach://%s", upload.Filename)
	}

	return form.addFilePart(upload.Filename, upload.Filename, upload.Data)
}

func addFormFieldCustomMarshal(form *requestForm, fieldName string, value customMarshal) error {
	line, errEncode := value.MarshalCustom()
	if errEncode != nil {
		return errEncode
	}

	return form.addFieldPart(fieldName, bytes.NewReader(line))
}

func addFormFieldInputMedia(form *requestForm, fieldName string, value inputMedia) error {
	if isNilValue(value) {
		return fmt.Errorf("nil value for field %s", fieldName)
	}

	line, err := addFormFieldInputMediaItem(form, value)
	if err != nil {
		return err
	}

	return form.addFieldPart(fieldName, bytes.NewReader(line))
}

func addFormFieldInputMediaSlice(form *requestForm, fieldName string, value []inputMedia) error {
	lines := acquireMediaBuffer()
	defer releaseMediaBuffer(lines)

	lines.WriteByte('[')
	for i, media := range value {
		if isNilValue(media) {
			return fmt.Errorf("nil value for field %s at index %d", fieldName, i)
		}

		line, err := addFormFieldInputMediaItem(form, media)
		if err != nil {
			return err
		}

		if i > 0 {
			lines.WriteByte(',')
		}

		lines.Write(line)
	}

	lines.WriteByte(']')
	return form.addFieldPart(fieldName, lines)
}

func addFormFieldInputRichMessage(form *requestForm, fieldName string, value *models.InputRichMessage) error {
	if value == nil {
		return fmt.Errorf("nil value for field %s", fieldName)
	}

	if err := addInputRichBlockSliceAttachments(form, value.Blocks); err != nil {
		return err
	}

	for i, media := range value.Media {
		if isNilValue(media.Media) {
			return fmt.Errorf("nil media for field %s at index %d", fieldName, i)
		}

		if err := addInputMediaAttachment(form, media.Media); err != nil {
			return err
		}
	}

	return addFormFieldDefault(form, fieldName, value)
}

func addInputRichBlockSliceAttachments(form *requestForm, blocks []models.InputRichBlock) error {
	for _, block := range blocks {
		if err := addInputRichBlockAttachments(form, block); err != nil {
			return err
		}
	}

	return nil
}

func addInputRichBlockAttachments(form *requestForm, block models.InputRichBlock) error {
	switch block.Type {
	case models.RichBlockTypeAnimation:
		if block.InputRichBlockAnimation != nil {
			return addInputMediaAttachment(form, &block.InputRichBlockAnimation.Animation)
		}
	case models.RichBlockTypeAudio:
		if block.InputRichBlockAudio != nil {
			return addInputMediaAttachment(form, &block.InputRichBlockAudio.Audio)
		}
	case models.RichBlockTypeDocument:
		if block.InputRichBlockDocument != nil {
			return addInputMediaAttachment(form, &block.InputRichBlockDocument.Document)
		}
	case models.RichBlockTypePhoto:
		if block.InputRichBlockPhoto != nil {
			return addInputMediaAttachment(form, &block.InputRichBlockPhoto.Photo)
		}
	case models.RichBlockTypeVideo:
		if block.InputRichBlockVideo != nil {
			return addInputMediaAttachment(form, &block.InputRichBlockVideo.Video)
		}
	case models.RichBlockTypeVoiceNote:
		if block.InputRichBlockVoiceNote != nil {
			return addInputMediaAttachment(form, &block.InputRichBlockVoiceNote.VoiceNote)
		}
	case models.RichBlockTypeList:
		if block.InputRichBlockList != nil {
			for _, item := range block.InputRichBlockList.Items {
				if err := addInputRichBlockSliceAttachments(form, item.Blocks); err != nil {
					return err
				}
			}
		}
	case models.RichBlockTypeBlockQuotation:
		if block.InputRichBlockBlockQuotation != nil {
			return addInputRichBlockSliceAttachments(form, block.InputRichBlockBlockQuotation.Blocks)
		}
	case models.RichBlockTypeCollage:
		if block.InputRichBlockCollage != nil {
			return addInputRichBlockSliceAttachments(form, block.InputRichBlockCollage.Blocks)
		}
	case models.RichBlockTypeSlideshow:
		if block.InputRichBlockSlideshow != nil {
			return addInputRichBlockSliceAttachments(form, block.InputRichBlockSlideshow.Blocks)
		}
	case models.RichBlockTypeDetails:
		if block.InputRichBlockDetails != nil {
			return addInputRichBlockSliceAttachments(form, block.InputRichBlockDetails.Blocks)
		}
	}

	return nil
}

func addFormFieldInlineQueryResultSlice(form *requestForm, fieldName string, value []models.InlineQueryResult) error {
	lines := acquireMediaBuffer()
	defer releaseMediaBuffer(lines)

	lines.WriteByte('[')
	for i, media := range value {
		if isNilValue(media) {
			return fmt.Errorf("nil value for field %s at index %d", fieldName, i)
		}

		line, errEncode := media.MarshalCustom()
		if errEncode != nil {
			return errEncode
		}

		if i > 0 {
			lines.WriteByte(',')
		}

		lines.Write(line)
	}

	lines.WriteByte(']')
	return form.addFieldPart(fieldName, lines)
}

func addFormFieldInputStickerSlice(form *requestForm, fieldName string, value []models.InputSticker) error {
	lines := acquireMediaBuffer()
	defer releaseMediaBuffer(lines)

	lines.WriteByte('[')
	for i, sticker := range value {
		if filename, ok := strings.CutPrefix(sticker.Sticker, "attach://"); ok {
			if !form.hasFilePart(filename) && readerIsNil(sticker.StickerAttachment) {
				return fmt.Errorf("nil StickerAttachment for attach://%s", filename)
			}

			if err := form.addFilePart(filename, filename, sticker.StickerAttachment); err != nil {
				return err
			}
		}

		line, errEncode := json.Marshal(sticker)
		if errEncode != nil {
			return errEncode
		}

		if i > 0 {
			lines.WriteByte(',')
		}

		lines.Write(line)
	}

	lines.WriteByte(']')
	return form.addFieldPart(fieldName, lines)
}

func addFormFieldDefault(form *requestForm, fieldName string, value any) error {
	d, errMarshal := json.Marshal(value)
	if errMarshal != nil {
		return errMarshal
	}

	d = bytes.Trim(d, "\"")
	return form.addFieldPart(fieldName, bytes.NewReader(d))
}

func addFormFieldString(form *requestForm, fieldName string, value string) error {
	return form.addFieldPart(fieldName, strings.NewReader(value))
}
