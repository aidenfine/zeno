package writer

import (
	"io"
	"zeno/src/resp"
)

func NewWriter(w io.Writer) *Writer {
	return &Writer{writter: w}
}
func (w *Writer) Write(v resp.Value) error {
	var bytes = v.Marshal()
	_, err := w.writter.Write(bytes)
	if err != nil {
		return err
	}
	return nil
}
