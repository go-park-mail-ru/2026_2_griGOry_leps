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
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func ptr[T any](v T) *T { return &v }

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	return buf.Bytes()
}

type profileTestEnv struct {
	uc       *ProfileUsecase
	users    *MockUserRepository
	sessions *MockSessionRepository
	resets   *MockPasswordResetRepository
	verifier *MockEmailVerifier
	storage  *MockFileStorage
	tx       *MockTransactor
	userID   int32
}

func newProfileTestEnv(t *testing.T) *profileTestEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	e := &profileTestEnv{users: NewMockUserRepository(ctrl), sessions: NewMockSessionRepository(ctrl), resets: NewMockPasswordResetRepository(ctrl), verifier: NewMockEmailVerifier(ctrl), storage: NewMockFileStorage(ctrl), tx: NewMockTransactor(ctrl), userID: 7}
	e.uc = NewProfileUsecase(e.users, e.sessions, e.resets, e.verifier, e.storage, e.tx)
	return e
}

func profileUser() domain.User {
	return domain.User{ID: 7, FirstName: "Anna", Email: "anna@goget.pw", Phone: "+79990000001"}
}

func expectProfileBefore(e *profileTestEnv, before domain.User) {
	expectTransaction(e.tx)
	gomock.InOrder(
		e.users.EXPECT().LockByID(gomock.Any(),
			e.userID).Return(nil),
		e.users.EXPECT().GetByID(gomock.Any(),
			e.userID).Return(before, nil),
	)
}

func expectProfileResult(e *profileTestEnv, upd domain.ProfileUpdate, result domain.Profile) {
	gomock.InOrder(
		e.users.EXPECT().UpdateProfile(gomock.Any(),
			e.userID, upd).Return(nil),
		e.users.EXPECT().GetProfile(gomock.Any(),
			e.userID).Return(result, nil),
	)
}

func expectPasswordChange(t *testing.T, e *profileTestEnv) {
	t.Helper()
	hash := passwordHash(t, "Secret123")
	e.users.EXPECT().GetByID(gomock.Any(), e.userID).Return(domain.User{ID: e.userID, PasswordHash: hash}, nil)
	expectTransaction(e.tx)
	gomock.InOrder(e.users.EXPECT().GetPasswordHashForUpdate(gomock.Any(), e.userID).Return(hash, nil),
		e.users.EXPECT().UpdatePassword(gomock.Any(), e.userID, gomock.Cond(func(hash string) bool {
			return bcrypt.CompareHashAndPassword([]byte(hash), []byte("NewSecret1")) == nil
		})).Return(nil),
		e.resets.EXPECT().UseAll(gomock.Any(), e.userID).Return(nil), e.users.EXPECT().SetPendingEmail(gomock.Any(), e.userID, nil).Return(nil), e.verifier.EXPECT().RevokeLinks(gomock.Any(), e.userID).Return(nil), e.sessions.EXPECT().DeleteOthers(gomock.Any(), e.userID, "current").Return(nil))
}

func expectAvatarSave(e *profileTestEnv, data []byte, extension string) *string {
	name := new(string)
	e.storage.EXPECT().Save(gomock.Any(), gomock.Cond(func(s string) bool { return len(s) == 32+len(extension) && strings.HasSuffix(s, extension) }), data).DoAndReturn(func(_ context.Context, s string, _ []byte) error { *name = s; return nil })
	return name
}

func expectAvatarUpdate(e *profileTestEnv, name *string, old string) {
	expectTransaction(e.tx)
	gomock.InOrder(
		e.users.EXPECT().GetAvatarForUpdate(gomock.Any(),
			e.userID).Return(old, nil),
		e.users.EXPECT().UpdateAvatar(gomock.Any(),
			e.userID, gomock.Cond(func(s string) bool { return s == *name })).Return(nil),
	)
	if old != "" {
		e.storage.EXPECT().Delete(gomock.Any(), old).Return(nil)
	}
}

func TestProfile_Get(t *testing.T) {
	e := newProfileTestEnv(t)
	want := domain.Profile{User: profileUser()}
	e.users.EXPECT().GetProfile(gomock.Any(), e.userID).Return(want, nil)
	got, err := e.uc.Profile(context.Background(), e.userID)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestUpdateProfile_OnlySentFieldsChange(t *testing.T) {
	e := newProfileTestEnv(t)
	before := profileUser()
	expectProfileBefore(e, before)
	upd := domain.ProfileUpdate{About: ptr("after move"), CityID: ptr(int32(1))}
	want := domain.Profile{User: before, About: "after move", City: &domain.City{ID: 1, Name: "Moscow"}}
	expectProfileResult(e, upd, want)
	upd.About = ptr("  after move  ")
	got, err := e.uc.UpdateProfile(context.Background(), e.userID, upd)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestUpdateProfile_NormalizesFields(t *testing.T) {
	e := newProfileTestEnv(t)
	expectProfileBefore(e, profileUser())
	e.verifier.EXPECT().IssueLink(gomock.Any(), e.userID, "anna@mail.ru").Return(func() {}, nil)
	e.resets.EXPECT().UseAll(gomock.Any(), e.userID).Return(nil)
	upd := domain.ProfileUpdate{FirstName: ptr("Anna Petrovna"), Phone: ptr("+79001234567"), Email: ptr("anna@mail.ru")}
	want := domain.Profile{User: domain.User{ID: 7, FirstName: *upd.FirstName, Phone: *upd.Phone, Email: *upd.Email}}
	expectProfileResult(e, upd, want)
	got, err := e.uc.UpdateProfile(context.Background(), e.userID, domain.ProfileUpdate{FirstName: ptr("  Anna   Petrovna "), Phone: ptr("8 (900) 123-45-67"), Email: ptr(" Anna@Mail.RU ")})
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestUpdateProfile_ClearAbout(t *testing.T) {
	e := newProfileTestEnv(t)
	expectProfileBefore(e, profileUser())
	expectProfileResult(e, domain.ProfileUpdate{About: ptr("")}, domain.Profile{User: profileUser()})
	got, err := e.uc.UpdateProfile(context.Background(), e.userID, domain.ProfileUpdate{About: ptr("")})
	require.NoError(t, err)
	require.Empty(t, got.About)
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
		{"city with digits", domain.ProfileUpdate{City: ptr("Москва 2")}, domain.ErrInvalidCityName},
		{"long city", domain.ProfileUpdate{City: ptr(strings.Repeat("я", 101))}, domain.ErrInvalidCityName},
		{"long about", domain.ProfileUpdate{About: ptr(strings.Repeat("я", maxAboutLength+1))}, domain.ErrAboutTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newProfileTestEnv(t)
			if tt.name == "unknown city" {
				expectProfileBefore(env, profileUser())
				env.users.EXPECT().UpdateProfile(gomock.Any(), env.userID, tt.upd).Return(domain.ErrInvalidCity)
			}

			_, err := env.uc.UpdateProfile(context.Background(), env.userID, tt.upd)

			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestUpdateProfile_CityByName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"  москва ", "Москва"}, {"нижний  тагил", "Нижний тагил"}, {" ", ""}} {
		t.Run(tc.in, func(t *testing.T) {
			e := newProfileTestEnv(t)
			expectProfileBefore(e, profileUser())
			result := domain.Profile{User: profileUser()}
			if tc.want != "" {
				result.City = &domain.City{ID: 1, Name: tc.want}
			}
			expectProfileResult(e, domain.ProfileUpdate{City: ptr(tc.want)}, result)
			got, err := e.uc.UpdateProfile(context.Background(), e.userID, domain.ProfileUpdate{City: ptr(tc.in)})
			require.NoError(t, err)
			require.Equal(t, result, got)
		})
	}
}

func TestUpdateProfile_AboutLimitInCharacters(t *testing.T) {
	e := newProfileTestEnv(t)
	expectProfileBefore(e, profileUser())
	upd := domain.ProfileUpdate{About: ptr(strings.Repeat("я", maxAboutLength))}
	expectProfileResult(e, upd, domain.Profile{About: *upd.About})
	got, err := e.uc.UpdateProfile(context.Background(), e.userID, upd)
	require.NoError(t, err)
	require.Equal(t, *upd.About, got.About)
}

func TestUpdateProfile_EmailTaken(t *testing.T) {
	e := newProfileTestEnv(t)
	expectProfileBefore(e, profileUser())
	e.verifier.EXPECT().IssueLink(gomock.Any(), e.userID, "other@goget.pw").Return(func() {}, nil)
	e.users.EXPECT().UpdateProfile(gomock.Any(), e.userID, domain.ProfileUpdate{Email: ptr("other@goget.pw")}).Return(domain.ErrEmailTaken)
	_, err := e.uc.UpdateProfile(context.Background(), e.userID, domain.ProfileUpdate{Email: ptr("other@goget.pw")})
	require.ErrorIs(t, err, domain.ErrEmailTaken)
}

func TestChangePassword_KeepsOnlyCurrentSession(t *testing.T) {
	e := newProfileTestEnv(t)
	expectPasswordChange(t, e)
	require.NoError(t, e.uc.ChangePassword(context.Background(), e.userID, "current", "Secret123", "NewSecret1"))
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	env := newProfileTestEnv(t)
	env.users.EXPECT().GetByID(gomock.Any(), env.userID).Return(domain.User{PasswordHash: passwordHash(t, "Secret123")}, nil).Times(1)

	err := env.uc.ChangePassword(context.Background(), env.userID, "current", "Wrong1234", "NewSecret1")

	require.ErrorIs(t, err, domain.ErrWrongPassword)
}

func TestChangePassword_WeakNew(t *testing.T) {
	env := newProfileTestEnv(t)
	env.users.EXPECT().GetByID(gomock.Any(), env.userID).Return(domain.User{PasswordHash: passwordHash(t, "Secret123")}, nil).Times(2)

	err := env.uc.ChangePassword(context.Background(), env.userID, "current", "Secret123", "short")
	require.ErrorIs(t, err, domain.ErrWeakPassword)

	err = env.uc.ChangePassword(context.Background(), env.userID, "current", "Secret123", strings.Repeat("Aa1", 30))
	require.ErrorIs(t, err, domain.ErrPasswordTooLong)
}

func TestChangePassword_TxError(t *testing.T) {
	e := newProfileTestEnv(t)
	txErr := errors.New("commit failed")
	e.users.EXPECT().GetByID(gomock.Any(), e.userID).Return(domain.User{PasswordHash: passwordHash(t, "Secret123")}, nil)
	e.tx.EXPECT().WithinTx(gomock.Any(), gomock.Any()).Return(txErr)
	require.ErrorIs(t, e.uc.ChangePassword(context.Background(), e.userID, "current", "Secret123", "NewSecret1"), txErr)
}

func TestUploadAvatar_SavesAndReplacesOld(t *testing.T) {
	e := newProfileTestEnv(t)
	data := pngBytes(t)
	firstName := expectAvatarSave(e, data, ".png")
	expectAvatarUpdate(e, firstName, "")
	first, err := e.uc.UploadAvatar(context.Background(), e.userID, data)
	require.NoError(t, err)
	require.Equal(t, *firstName, first)
	secondName := expectAvatarSave(e, data, ".png")
	expectAvatarUpdate(e, secondName, first)
	second, err := e.uc.UploadAvatar(context.Background(), e.userID, data)
	require.NoError(t, err)
	require.Equal(t, *secondName, second)
	require.NotEqual(t, first, second)
}

func TestUploadAvatar_JPEGExtension(t *testing.T) {
	env := newProfileTestEnv(t)
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil))

	saved := expectAvatarSave(env, buf.Bytes(), ".jpg")
	expectAvatarUpdate(env, saved, "")
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
			env := newProfileTestEnv(t)

			_, err := env.uc.UploadAvatar(context.Background(), env.userID, tt.data)

			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestUploadAvatar_BrokenFileKeepsOldAvatar(t *testing.T) {
	e := newProfileTestEnv(t)
	full := noisyPNG(t)
	_, err := e.uc.UploadAvatar(context.Background(), e.userID, full[:len(full)/2])
	require.ErrorIs(t, err, domain.ErrInvalidImage)
}

func TestUploadAvatar_RemovesFileWhenUserMissing(t *testing.T) {
	e := newProfileTestEnv(t)
	data := pngBytes(t)
	saved := expectAvatarSave(e, data, ".png")
	expectTransaction(e.tx)
	e.users.EXPECT().GetAvatarForUpdate(gomock.Any(), int32(8)).Return("", domain.ErrUserNotFound)
	e.storage.EXPECT().Delete(gomock.Any(), gomock.Cond(func(name string) bool { return name == *saved })).Return(nil)
	_, err := e.uc.UploadAvatar(context.Background(), 8, data)
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestUploadAvatar_StorageError(t *testing.T) {
	e := newProfileTestEnv(t)
	data := pngBytes(t)
	diskErr := errors.New("disk full")
	e.storage.EXPECT().Save(gomock.Any(), gomock.Any(), data).Return(diskErr)
	_, err := e.uc.UploadAvatar(context.Background(), e.userID, data)
	require.ErrorIs(t, err, diskErr)
}

func TestUploadAvatar_TxErrorRemovesNewFile(t *testing.T) {
	e := newProfileTestEnv(t)
	data := pngBytes(t)
	saved := expectAvatarSave(e, data, ".png")
	txErr := errors.New("commit failed")
	e.tx.EXPECT().WithinTx(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		if err := fn(ctx); err != nil {
			return err
		}
		return txErr
	})
	e.users.EXPECT().GetAvatarForUpdate(gomock.Any(), e.userID).Return("old.png", nil)
	e.users.EXPECT().UpdateAvatar(gomock.Any(), e.userID, gomock.Cond(func(name string) bool { return name == *saved })).Return(nil)
	e.storage.EXPECT().Delete(gomock.Any(), gomock.Cond(func(name string) bool { return name == *saved })).Return(nil)
	_, err := e.uc.UploadAvatar(context.Background(), e.userID, data)
	require.ErrorIs(t, err, txErr)
}

func TestChangePassword_PasswordChangedConcurrently(t *testing.T) {
	e := newProfileTestEnv(t)
	e.users.EXPECT().GetByID(gomock.Any(), e.userID).Return(domain.User{PasswordHash: passwordHash(t, "Secret123")}, nil)
	expectTransaction(e.tx)
	e.users.EXPECT().GetPasswordHashForUpdate(gomock.Any(), e.userID).Return("hash-after-change", nil)
	require.ErrorIs(t, e.uc.ChangePassword(context.Background(), e.userID, "current", "Secret123", "NewSecret1"), domain.ErrWrongPassword)
}

func TestChangePassword_RevokesResetLinks(t *testing.T) {
	e := newProfileTestEnv(t)
	expectPasswordChange(t, e)
	require.NoError(t, e.uc.ChangePassword(context.Background(), e.userID, "current", "Secret123", "NewSecret1"))
}

func TestUpdateProfile_NewEmailRevokesResetLinks(t *testing.T) {
	e := newProfileTestEnv(t)
	expectProfileBefore(e, profileUser())
	e.verifier.EXPECT().IssueLink(gomock.Any(), e.userID, "new@goget.pw").Return(func() {}, nil)
	e.resets.EXPECT().UseAll(gomock.Any(), e.userID).Return(nil)
	expectProfileResult(e, domain.ProfileUpdate{Email: ptr("new@goget.pw")}, domain.Profile{})
	_, err := e.uc.UpdateProfile(context.Background(), e.userID, domain.ProfileUpdate{Email: ptr("new@goget.pw")})
	require.NoError(t, err)
}

func TestUpdateProfile_SameEmailKeepsResetLinks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		in, want domain.ProfileUpdate
	}{
		{"same email", domain.ProfileUpdate{Email: ptr(" Anna@GoGet.pw ")}, domain.ProfileUpdate{}},
		{"other field", domain.ProfileUpdate{About: ptr("sale")}, domain.ProfileUpdate{About: ptr("sale")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newProfileTestEnv(t)
			expectProfileBefore(e, profileUser())
			expectProfileResult(e, tc.want, domain.Profile{})
			_, err := e.uc.UpdateProfile(context.Background(), e.userID, tc.in)
			require.NoError(t, err)
		})
	}
}
