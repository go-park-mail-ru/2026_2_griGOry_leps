package usecase

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type storageFake struct {
	mu      sync.Mutex
	files   map[string][]byte
	saveErr error
}

func newStorageFake() *storageFake {
	return &storageFake{files: make(map[string][]byte)}
}

func (s *storageFake) Save(_ context.Context, name string, data []byte) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[name] = data
	return nil
}

func (s *storageFake) Delete(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, name)
	return nil
}

type profileEnv struct {
	uc       *ProfileUsecase
	users    *userRepoFake
	sessions *sessionRepoFake
	resets   *resetRepoFake
	verifier *verifierFake
	storage  *storageFake
	userID   int32
}

func newProfileEnv(t *testing.T) profileEnv {
	t.Helper()
	users := newUserRepoFake()
	sessions := newSessionRepoFake()
	resets := newResetRepoFake()
	storage := newStorageFake()
	verifier := &verifierFake{}

	hash, err := bcrypt.GenerateFromPassword([]byte("Secret123"), bcrypt.MinCost)
	require.NoError(t, err)
	user, err := users.Create(context.Background(), "anna@goget.pw", string(hash), "Анна", "+79990000001")
	require.NoError(t, err)

	return profileEnv{
		uc:       NewProfileUsecase(users, sessions, resets, verifier, storage, txFake{}),
		users:    users,
		sessions: sessions,
		resets:   resets,
		verifier: verifier,
		storage:  storage,
		userID:   user.ID,
	}
}

func ptr[T any](v T) *T { return &v }

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	return buf.Bytes()
}

func TestProfile_Get(t *testing.T) {
	env := newProfileEnv(t)

	p, err := env.uc.Profile(context.Background(), env.userID)

	require.NoError(t, err)
	require.Equal(t, "Анна", p.FirstName)
	require.Empty(t, p.PasswordHash)
	require.Nil(t, p.City)
}

func TestUpdateProfile_OnlySentFieldsChange(t *testing.T) {
	env := newProfileEnv(t)

	p, err := env.uc.UpdateProfile(context.Background(), env.userID, domain.ProfileUpdate{
		About:  ptr("  Продаю вещи после переезда  "),
		CityID: ptr(int32(1)),
	})

	require.NoError(t, err)
	require.Equal(t, "Продаю вещи после переезда", p.About)
	require.Equal(t, &domain.City{ID: 1, Name: "Москва"}, p.City)
	require.Equal(t, "Анна", p.FirstName)
	require.Equal(t, "anna@goget.pw", p.Email)
}

func TestUpdateProfile_NormalizesFields(t *testing.T) {
	env := newProfileEnv(t)

	p, err := env.uc.UpdateProfile(context.Background(), env.userID, domain.ProfileUpdate{
		FirstName: ptr("  Анна   Петровна "),
		Phone:     ptr("8 (900) 123-45-67"),
		Email:     ptr(" Anna@Mail.RU "),
	})

	require.NoError(t, err)
	require.Equal(t, "Анна Петровна", p.FirstName)
	require.Equal(t, "+79001234567", p.Phone)
	require.Equal(t, "anna@mail.ru", p.Email)
}

func TestUpdateProfile_ClearAbout(t *testing.T) {
	env := newProfileEnv(t)
	_, err := env.uc.UpdateProfile(context.Background(), env.userID, domain.ProfileUpdate{About: ptr("было")})
	require.NoError(t, err)

	p, err := env.uc.UpdateProfile(context.Background(), env.userID, domain.ProfileUpdate{About: ptr("")})

	require.NoError(t, err)
	require.Empty(t, p.About)
}

func TestUpdateProfile_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		upd  domain.ProfileUpdate
		want error
	}{
		{"empty name", domain.ProfileUpdate{FirstName: ptr("   ")}, domain.ErrMissingFirstName},
		{"bad name", domain.ProfileUpdate{FirstName: ptr("<script>")}, domain.ErrInvalidFirstName},
		{"bad phone", domain.ProfileUpdate{Phone: ptr("123")}, domain.ErrInvalidPhone},
		{"bad email", domain.ProfileUpdate{Email: ptr("not-an-email")}, domain.ErrInvalidEmail},
		{"zero city", domain.ProfileUpdate{CityID: ptr(int32(0))}, domain.ErrInvalidCity},
		{"unknown city", domain.ProfileUpdate{CityID: ptr(int32(99))}, domain.ErrInvalidCity},
		{"long about", domain.ProfileUpdate{About: ptr(strings.Repeat("я", maxAboutLength+1))}, domain.ErrAboutTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newProfileEnv(t)

			_, err := env.uc.UpdateProfile(context.Background(), env.userID, tt.upd)

			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestUpdateProfile_AboutLimitInCharacters(t *testing.T) {
	env := newProfileEnv(t)

	_, err := env.uc.UpdateProfile(context.Background(), env.userID, domain.ProfileUpdate{About: ptr(strings.Repeat("я", maxAboutLength))})

	require.NoError(t, err)
}

func TestUpdateProfile_EmailTaken(t *testing.T) {
	env := newProfileEnv(t)
	_, err := env.users.Create(context.Background(), "other@goget.pw", "hash", "Олег", "+79990000002")
	require.NoError(t, err)

	_, err = env.uc.UpdateProfile(context.Background(), env.userID, domain.ProfileUpdate{Email: ptr("other@goget.pw")})

	require.ErrorIs(t, err, domain.ErrEmailTaken)
}

func addSession(t *testing.T, env profileEnv, id string, userID int32) {
	t.Helper()
	require.NoError(t, env.sessions.Create(context.Background(), id, userID, time.Now().Add(time.Hour)))
}

func TestChangePassword_KeepsOnlyCurrentSession(t *testing.T) {
	env := newProfileEnv(t)
	addSession(t, env, "current", env.userID)
	addSession(t, env, "phone", env.userID)
	addSession(t, env, "stranger", env.userID+100)

	err := env.uc.ChangePassword(context.Background(), env.userID, "current", "Secret123", "NewSecret1")
	require.NoError(t, err)

	user, err := env.users.GetByID(context.Background(), env.userID)
	require.NoError(t, err)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("NewSecret1")))

	_, err = env.sessions.GetByID(context.Background(), "current")
	require.NoError(t, err)
	_, err = env.sessions.GetByID(context.Background(), "phone")
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
	_, err = env.sessions.GetByID(context.Background(), "stranger")
	require.NoError(t, err)
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	env := newProfileEnv(t)

	err := env.uc.ChangePassword(context.Background(), env.userID, "current", "Wrong1234", "NewSecret1")

	require.ErrorIs(t, err, domain.ErrWrongPassword)
}

func TestChangePassword_WeakNew(t *testing.T) {
	env := newProfileEnv(t)

	err := env.uc.ChangePassword(context.Background(), env.userID, "current", "Secret123", "short")
	require.ErrorIs(t, err, domain.ErrWeakPassword)

	err = env.uc.ChangePassword(context.Background(), env.userID, "current", "Secret123", strings.Repeat("Aa1", 30))
	require.ErrorIs(t, err, domain.ErrPasswordTooLong)
}

func TestChangePassword_TxError(t *testing.T) {
	env := newProfileEnv(t)
	txErr := errors.New("commit failed")
	uc := NewProfileUsecase(env.users, env.sessions, env.resets, env.verifier, env.storage, failingTx{err: txErr})

	err := uc.ChangePassword(context.Background(), env.userID, "current", "Secret123", "NewSecret1")

	require.ErrorIs(t, err, txErr)
}

func TestUploadAvatar_SavesAndReplacesOld(t *testing.T) {
	env := newProfileEnv(t)
	data := pngBytes(t)

	first, err := env.uc.UploadAvatar(context.Background(), env.userID, data)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(first, ".png"))
	require.Equal(t, data, env.storage.files[first])

	second, err := env.uc.UploadAvatar(context.Background(), env.userID, data)
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	require.NotContains(t, env.storage.files, first)
	require.Contains(t, env.storage.files, second)

	p, err := env.uc.Profile(context.Background(), env.userID)
	require.NoError(t, err)
	require.Equal(t, second, p.Avatar)
}

func TestUploadAvatar_JPEGExtension(t *testing.T) {
	env := newProfileEnv(t)
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil))

	name, err := env.uc.UploadAvatar(context.Background(), env.userID, buf.Bytes())

	require.NoError(t, err)
	require.True(t, strings.HasSuffix(name, ".jpg"))
}

func pngHeader(width, height uint32) []byte {
	ihdr := make([]byte, 17)
	copy(ihdr, "IHDR")
	binary.BigEndian.PutUint32(ihdr[4:], width)
	binary.BigEndian.PutUint32(ihdr[8:], height)
	ihdr[12] = 8
	ihdr[13] = 2

	data := []byte("\x89PNG\r\n\x1a\n")
	data = binary.BigEndian.AppendUint32(data, 13)
	data = append(data, ihdr...)
	return binary.BigEndian.AppendUint32(data, crc32.ChecksumIEEE(ihdr))
}

func noisyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 31)
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestUploadAvatar_RejectsBadFiles(t *testing.T) {
	full := noisyPNG(t)

	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"text", []byte("hello, it's not an image"), domain.ErrUnsupportedImage},
		{"gif", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00"), domain.ErrUnsupportedImage},
		{"broken header", full[:12], domain.ErrInvalidImage},
		{"only header", pngHeader(10, 10), domain.ErrInvalidImage},
		{"cut in the middle", full[:len(full)/2], domain.ErrInvalidImage},
		{"too many pixels", pngHeader(5000, 5000), domain.ErrImageTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newProfileEnv(t)

			_, err := env.uc.UploadAvatar(context.Background(), env.userID, tt.data)

			require.ErrorIs(t, err, tt.want)
			require.Empty(t, env.storage.files)
		})
	}
}

func TestUploadAvatar_BrokenFileKeepsOldAvatar(t *testing.T) {
	env := newProfileEnv(t)
	old, err := env.uc.UploadAvatar(context.Background(), env.userID, pngBytes(t))
	require.NoError(t, err)

	full := noisyPNG(t)
	_, err = env.uc.UploadAvatar(context.Background(), env.userID, full[:len(full)/2])
	require.ErrorIs(t, err, domain.ErrInvalidImage)

	p, err := env.uc.Profile(context.Background(), env.userID)
	require.NoError(t, err)
	require.Equal(t, old, p.Avatar)
	require.Contains(t, env.storage.files, old)
}

func TestUploadAvatar_RemovesFileWhenUserMissing(t *testing.T) {
	env := newProfileEnv(t)

	_, err := env.uc.UploadAvatar(context.Background(), env.userID+1, pngBytes(t))

	require.ErrorIs(t, err, domain.ErrUserNotFound)
	require.Empty(t, env.storage.files)
}

func TestUploadAvatar_StorageError(t *testing.T) {
	env := newProfileEnv(t)
	env.storage.saveErr = errors.New("disk full")

	_, err := env.uc.UploadAvatar(context.Background(), env.userID, pngBytes(t))

	require.ErrorIs(t, err, env.storage.saveErr)
	p, err := env.uc.Profile(context.Background(), env.userID)
	require.NoError(t, err)
	require.Empty(t, p.Avatar)
}

func TestUploadAvatar_TxErrorRemovesNewFile(t *testing.T) {
	env := newProfileEnv(t)
	txErr := errors.New("commit failed")
	uc := NewProfileUsecase(env.users, env.sessions, env.resets, env.verifier, env.storage, failingTx{err: txErr})

	_, err := uc.UploadAvatar(context.Background(), env.userID, pngBytes(t))

	require.ErrorIs(t, err, txErr)
	require.Empty(t, env.storage.files)
}

func TestChangePassword_PasswordChangedConcurrently(t *testing.T) {
	env := newProfileEnv(t)
	addSession(t, env, "current", env.userID)
	addSession(t, env, "phone", env.userID)
	before, err := env.users.GetByID(context.Background(), env.userID)
	require.NoError(t, err)
	uc := NewProfileUsecase(passwordChangedRepo{env.users}, env.sessions, env.resets, env.verifier, env.storage, txFake{})

	err = uc.ChangePassword(context.Background(), env.userID, "current", "Secret123", "NewSecret1")

	require.ErrorIs(t, err, domain.ErrWrongPassword)
	after, err := env.users.GetByID(context.Background(), env.userID)
	require.NoError(t, err)
	require.Equal(t, before.PasswordHash, after.PasswordHash)
	_, err = env.sessions.GetByID(context.Background(), "phone")
	require.NoError(t, err)
}

func (env profileEnv) addResetToken(t *testing.T, token string) {
	t.Helper()
	require.NoError(t, env.resets.Create(context.Background(), hashToken(token), env.userID, time.Now().Add(time.Hour)))
}

func (env profileEnv) resetTokenUsed(t *testing.T, token string) bool {
	t.Helper()
	reset, err := env.resets.Get(context.Background(), hashToken(token))
	require.NoError(t, err)
	return reset.UsedAt != nil
}

func TestChangePassword_RevokesResetLinks(t *testing.T) {
	env := newProfileEnv(t)
	env.addResetToken(t, "old-link")

	require.NoError(t, env.uc.ChangePassword(context.Background(), env.userID, "current", "Secret123", "NewSecret1"))

	require.True(t, env.resetTokenUsed(t, "old-link"))
}

func TestUpdateProfile_NewEmailRevokesResetLinks(t *testing.T) {
	env := newProfileEnv(t)
	env.addResetToken(t, "old-link")

	_, err := env.uc.UpdateProfile(context.Background(), env.userID, domain.ProfileUpdate{Email: ptr("new@goget.pw")})

	require.NoError(t, err)
	require.True(t, env.resetTokenUsed(t, "old-link"))
}

func TestUpdateProfile_SameEmailKeepsResetLinks(t *testing.T) {
	tests := map[string]domain.ProfileUpdate{
		"same email":  {Email: ptr(" Anna@GoGet.pw ")},
		"other field": {About: ptr("Продаю вещи")},
	}

	for name, upd := range tests {
		t.Run(name, func(t *testing.T) {
			env := newProfileEnv(t)
			env.addResetToken(t, "link")

			_, err := env.uc.UpdateProfile(context.Background(), env.userID, upd)

			require.NoError(t, err)
			require.False(t, env.resetTokenUsed(t, "link"))
		})
	}
}
