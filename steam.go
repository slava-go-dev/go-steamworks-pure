package go_steamworks_pure

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

const kNCubTicketMaxLength = 2560

type HSteamPipe uint32
type HSteamUser uint32

type CallbackT struct {
	HSteamUser HSteamUser     // Steam user handle
	Callback   int32          // Callback ID
	ParamPtr   unsafe.Pointer // Pointer to callback parameters
	ParamSize  int32          // Size of parameters
}

type ISteamUser struct{}

type ISteamApps struct{}

type ISteamFriends struct{}

var (
	SteamapiRestartAppIfNecessary       func(steamAppId uint32) bool
	SteamapiInitFlat                    func(*byte) int
	SteamapiIsSteamRunning              func() bool
	SteamapiReleaseCurrentThreadMemory  func()
	SteamapiShutdown                    func()
	SteamapiISteamAppsBIsDlcInstalled   func(self *ISteamApps, appID uint32) bool
	GetSteamApps                        func() *ISteamApps
	GetSteamUser                        func() *ISteamUser
	GetSteamID                          func(self *ISteamUser) uint64
	GetAuthTicketForWebApi              func(self *ISteamUser, pchIdentity *byte) uint32
	ISteamUserBLoggedOn                 func(self *ISteamUser) bool
	GetSteamFriends                     func() *ISteamFriends
	SteamapiISteamFriendsGetPersonaName func(self *ISteamFriends) string
	CancelAuthTicket                    func(self *ISteamUser, ticket uint32) uint32
	SteamapiGethsteamuser               func() HSteamUser
	GetCurrentGameLanguage              func(self *ISteamApps) string
	UserHasLicenseForApp                func(self *ISteamUser, steamId uint64, appID uint32) int
)

type logger interface {
	Infof(msg string, args ...interface{})
	Debugf(msg string, args ...interface{})
	Errorf(msg string, args ...interface{})
	Warnf(msg string, args ...interface{})
}

type Steam struct {
	user         *ISteamUser
	steamApps    *ISteamApps
	steamFriends *ISteamFriends
	steamUser    HSteamUser
	libc         uintptr
	logger       logger
	dispatcher   *ManualDispatcher
	inited       bool
}

func New(appID uint32, libsPath string, logger logger) (*Steam, error) {
	libc, err := loadSystemLib(getSystemLibrary(libsPath))
	if err != nil {
		return nil, fmt.Errorf("failed to load Steam library: %w", err)
	}

	if err := registerSteamFunctions(libc); err != nil {
		return nil, err
	}

	if SteamapiRestartAppIfNecessary(appID) {
		return nil, fmt.Errorf("application restarting through Steam")
	}

	var errMsg [256]byte
	result := SteamapiInitFlat(&errMsg[0])
	if result != 0 {
		errStr := string(errMsg[:])
		return nil, fmt.Errorf("SteamAPI_InitFlat returned error %d: %s", result, errStr)
	}

	steamUser := GetSteamUser()
	if steamUser == nil {
		SteamapiShutdown()
		return nil, fmt.Errorf("failed to get Steam user interface")
	}
	steamApps := GetSteamApps()
	if steamApps == nil {
		SteamapiShutdown()
		return nil, fmt.Errorf("failed to get Steam apps interface")
	}
	steamFriends := GetSteamFriends()
	if steamFriends == nil {
		SteamapiShutdown()
		return nil, fmt.Errorf("failed to get Steam friends interface")
	}

	// Initialize manual dispatch
	registerManualDispatch(libc)
	SteamapiManualdispatchInit()

	hsUser := SteamapiGethsteamuser()

	steam := &Steam{
		user:         steamUser,
		steamApps:    steamApps,
		steamFriends: steamFriends,
		steamUser:    hsUser,
		libc:         libc,
		logger:       logger,
		dispatcher:   NewManualDispatcher(logger),
		inited:       true,
	}

	logger.Infof("Steam successfully initialized")
	return steam, nil
}

// StartCallbackDispatcher starts the dispatcher for handling callbacks
func (s *Steam) StartCallbackDispatcher(ctx context.Context) {
	if s.dispatcher != nil {
		s.dispatcher.Start(ctx)
	}
}

func registerSteamFunctions(libc uintptr) error {
	purego.RegisterLibFunc(&SteamapiRestartAppIfNecessary, libc, "SteamAPI_RestartAppIfNecessary")
	purego.RegisterLibFunc(&SteamapiInitFlat, libc, "SteamAPI_InitFlat")
	purego.RegisterLibFunc(&SteamapiIsSteamRunning, libc, "SteamAPI_IsSteamRunning")
	purego.RegisterLibFunc(&SteamapiReleaseCurrentThreadMemory, libc, "SteamAPI_ReleaseCurrentThreadMemory")
	purego.RegisterLibFunc(&SteamapiShutdown, libc, "SteamAPI_Shutdown")
	purego.RegisterLibFunc(&GetSteamUser, libc, "SteamAPI_SteamUser_v023")
	purego.RegisterLibFunc(&GetAuthTicketForWebApi, libc, "SteamAPI_ISteamUser_GetAuthTicketForWebApi")
	purego.RegisterLibFunc(&CancelAuthTicket, libc, "SteamAPI_ISteamUser_CancelAuthTicket")
	purego.RegisterLibFunc(&UserHasLicenseForApp, libc, "SteamAPI_ISteamUser_UserHasLicenseForApp")

	purego.RegisterLibFunc(&ISteamUserBLoggedOn, libc, "SteamAPI_ISteamUser_BLoggedOn")
	purego.RegisterLibFunc(&GetSteamApps, libc, "SteamAPI_SteamApps_v008")
	purego.RegisterLibFunc(&SteamapiISteamAppsBIsDlcInstalled, libc, "SteamAPI_ISteamApps_BIsDlcInstalled")
	purego.RegisterLibFunc(&GetCurrentGameLanguage, libc, "SteamAPI_ISteamApps_GetCurrentGameLanguage")
	purego.RegisterLibFunc(&GetSteamFriends, libc, "SteamAPI_SteamFriends_v017")
	purego.RegisterLibFunc(&SteamapiISteamFriendsGetPersonaName, libc, "SteamAPI_ISteamFriends_GetPersonaName")
	purego.RegisterLibFunc(&GetSteamID, libc, "SteamAPI_ISteamUser_GetSteamID")

	purego.RegisterLibFunc(&SteamapiGethsteamuser, libc, "SteamAPI_GetHSteamUser")

	return nil
}

func (s *Steam) IsSteamInitialized() bool {
	return s.user != nil && ISteamUserBLoggedOn(s.user)
}

func (s *Steam) IsSteamDlcInstalled(dlcAppId uint32) bool {
	return s.steamApps != nil && SteamapiISteamAppsBIsDlcInstalled(s.steamApps, dlcAppId)
}

func (s *Steam) GetSteamID() uint64 {
	if s.user == nil {
		return 0
	}
	return GetSteamID(s.user)
}

func (s *Steam) GetSteamName() string {
	if s.steamFriends == nil {
		return ""
	}
	return SteamapiISteamFriendsGetPersonaName(s.steamFriends)
}

func (s *Steam) GetSteamLanguage() string {
	if s.steamApps == nil {
		return ""
	}
	return GetCurrentGameLanguage(s.steamApps)
}

// GetAuthTicket gets an auth ticket and waits for the callback
func (s *Steam) GetAuthTicket(ctx context.Context, identity []byte, lifetime time.Duration) ([]byte, error) {
	if !s.dispatcher.IsRunning() {
		return nil, fmt.Errorf("dispatcher is not running")
	}
	if len(identity) == 0 {
		return nil, fmt.Errorf("identity is empty")
	}

	var identityPtr *byte
	identityPtr = &identity[0]

	resultChan := make(chan []byte, 1)
	errorChan := make(chan error, 1)

	var handle uint32

	err := s.dispatcher.RegisterCallback("auth", GetTicketForWebApiCallback, func(ctx context.Context, msg *CallbackT) bool {
		type GetTicketForWebApiResponse struct {
			HAuthTicket uint32                     // m_hAuthTicket
			EResult     int32                      // m_eResult
			CubTicket   int32                      // m_cubTicket
			RgubTicket  [kNCubTicketMaxLength]byte // m_rgubTicket
		}

		response := (*GetTicketForWebApiResponse)(msg.ParamPtr)

		if response.HAuthTicket != handle {
			return false
		}

		if response.EResult != 1 {
			errorChan <- fmt.Errorf("steam auth ticket request failed: %d", response.EResult)
			return true
		}

		if response.CubTicket == 0 {
			errorChan <- errors.New("invalid ticket size")
			return true
		}

		ticket := make([]byte, response.CubTicket)
		copy(ticket, response.RgubTicket[:response.CubTicket])
		resultChan <- ticket
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("failed to register callback: %w", err)
	}

	defer s.dispatcher.UnregisterCallback("auth")

	handle = GetAuthTicketForWebApi(s.user, identityPtr)
	if handle == 0 {
		return nil, fmt.Errorf("failed to get auth ticket handle")
	}

	cancelTimer := time.AfterFunc(lifetime, func() {
		s.CancelAuthTicket(handle)
	})
	defer cancelTimer.Stop()

	select {
	case ticket := <-resultChan:
		return ticket, nil

	case err := <-errorChan:
		s.CancelAuthTicket(handle)
		return nil, err

	case <-ctx.Done():
		s.CancelAuthTicket(handle)
		return nil, ctx.Err()
	}
}

func (s *Steam) CancelAuthTicket(handle uint32) {
	if s.inited {
		CancelAuthTicket(s.user, handle)
	}
}

func (s *Steam) Shutdown() {
	if s.dispatcher != nil {
		s.dispatcher.Close()
	}
	if SteamapiShutdown != nil {
		SteamapiShutdown()
	}
	s.inited = false
	closeLibc(s.libc)
}

func getSystemLibrary(libsPath string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(libsPath, "/libs/libsteam_api.dylib")
	case "linux":
		return filepath.Join(libsPath, "/libs/libsteam_api64.so")
	case "windows":
		return filepath.Join(libsPath, "/libs/steam_api64.dll")
	default:
		panic(fmt.Errorf("GOOS=%s is not supported", runtime.GOOS))
	}
}
