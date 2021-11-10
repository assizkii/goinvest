package redis

import (
	"bytes"
	"encoding/json"
)

// Serialize performs the following logic:
//   - If value is a byte array, it is returned as-is
//   - Else, jsoniter is used to serialize.
func Serialize(value interface{}) ([]byte, error) {

	if data, ok := value.([]byte); ok {
		return data, nil
	}

	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Deserialize transforms bytes produced by Serialize back into a Go object,
// storing it into "ptr", which must be a pointer to the value type.
func Deserialize(byt []byte, ptr interface{}) error {

	if data, ok := ptr.(*[]byte); ok {
		*data = byt
		return nil
	}

	b := bytes.NewBuffer(byt)
	decoder := json.NewDecoder(b)
	if err := decoder.Decode(ptr); err != nil {
		return err
	}

	return nil
}
