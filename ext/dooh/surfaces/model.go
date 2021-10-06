package surfaces

import (
	"encoding/json"
	"time"

	"github.com/jfk9w-go/flu"
)

type EmailAuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type TokenResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}

type Time time.Time

func (t *Time) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	var err error
	*(*time.Time)(t), err = time.Parse(time.RFC3339, str)
	return err
}

func (t *Time) GobDecode(data []byte) error {
	return flu.DecodeFrom(flu.Bytes(data), flu.Gob((*time.Time)(t)))
}

func (t Time) GobEncode() ([]byte, error) {
	buf := new(flu.ByteBuffer)
	if err := flu.EncodeTo(flu.Gob(time.Time(t)), buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (t Time) String() string {
	return time.Time(t).String()
}

type Surface struct {
	ID         string `json:"id"`
	Attributes struct {
		Name      string `json:"name"`
		Network   string `json:"network"`
		SurfaceID string `json:"surfaceId"`
		CreatedAt Time   `json:"createdAt"`
		UpdatedAt Time   `json:"updatedAt"`
		DeletedAt *Time  `json:"deletedAt"`
	}
}
