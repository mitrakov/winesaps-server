package user

import "sync"
import "time"
import "strings"
import "encoding/json"
import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint
import "mitrakov.ru/home/winesaps/checker"

// IUserManager is an interface for all user management operations
type IUserManager interface {
    SetController(controller IController)
    SignUp(name, email, password, agentInfo, promocode string) (*User, *Error)
    SignIn(name, password, agentInfo string) (*User, *Error, Sid)
    SignOut(user *User)
    GetUserByName(name string) (*User, bool) // go has no overloaded functions
    GetUserByID(id uint64) (*User, bool)
    GetUserBySid(sid Sid) (*User, bool)
    GetUserInfo(user *User) ([]byte, *Error)
    GetUserFriends(user *User) ([]byte, []string, *Error)
    AddFriend(user *User, name string) (character byte, err *Error)
    RemoveFriend(user *User, name string) *Error
    Accept(user1, user2 *User) *Error
    GetUserAbilities(user *User) ([]byte, *Error)
    RewardUsers(winnerSid, loserSid Sid, score1, score2 byte, trust bool, box *MailBox) (reward uint32, err *Error)
    ChangeCharacter(user *User, character byte) *Error
    ChangePassword(user *User, oldPassword, newPassword string) *Error
    GetAllAbilities() ([]byte, *Error)
    BuyProduct(user *User, code, days byte) *Error
    GetWins(user *User) (uint32, *Error) // not used since 1.3.8
    GetRating(user *User, ratingType byte) ([]byte, *Error)
    IsPromocodeValid(promocode string) (inviter *User, ok bool, err *Error)
    GetUsersCount() uint
    GetUsersCountTotal() uint
    GetUserNumberN(n uint) (*User, *Error)
    GetSkuGems() map[string]uint32
    CheckPayment(user *User, jsonStr, signature string) (gems uint32, box *MailBox, err *Error)
    Close()
}

// IDbManager is an interface to interact with a Database
type IDbManager interface {
    AddUser(name, email, hash, salt, promocode string) *Error
    GetUserByID(userID uint64) (*User, *Error)
    GetUserByName(name string) (*User, *Error)
    GetUserByNumber(number uint) (*User, *Error)
    GetAllAbilities() ([]byte, *Error)
    RegisterWin(ratingType byte, userID uint64, scoreDiff byte) *Error
    RegisterLoss(ratingType byte, userID uint64, scoreDiff byte) *Error
    RewardUser(userID uint64, gems, trustPoints uint32) *Error
    ConsumeTrustPoints(userID uint64, trustPoints uint32) *Error
    ChangeUser(userID uint64, email, hash string, character byte) *Error
    SetLastEnemy(userID, enemyID uint64) *Error
    SetAgentInfo(userID uint64, agentInfo string) *Error
    GetAbilities(userID uint64) ([]byte, []time.Time, *Error)
    BuyProduct(userID uint64, code, days byte) (cost uint32, error *Error)
    GetWins(userID uint64) (uint32, *Error) // not used since 1.3.8
    GetRating(userID uint64, ratingType, limit byte) ([]byte, *Error)
    GetBestUsers(ratingType, limit byte) (ids []uint64, error *Error)
    ClearRating(ratingType byte) *Error
    GetUserFriends(userID uint64) ([]byte, []string, *Error)
    AddFriend(userID uint64, name string) (character byte, err *Error)
    RemoveFriend(userID uint64, name string) *Error
    ActivatePromocode(userID, inviterID uint64) *Error
    DeactivatePromocode(userID, inviterID uint64) *Error
    PromocodeExists(userID uint64) (inviterID uint64, exists bool, err *Error)
    DeleteExpiredAbilities() (removedIds []uint64, error *Error)
    AddPayment(userID uint64, orderID, sku string, timestampMsec int64, data string, state uint8) *Error
    SetPaymentChecked(orderID string) *Error
    SetPaymentResult(orderID string, gems uint32) *Error
    Close() *Error
}

// IPacker interface comprises of methods for converting some events into a bytearray
type IPacker interface {
    PackUserInfo(info []byte) []byte
    PackPromocodeDone(inviter bool, name string, gems uint32) []byte
}

// IController contains methods for IUserManager callbacks
type IController interface {
    Event(*MailBox, *Error)
}

// androidPaymentT is a helper structure for Google InApp Purchase record.
// Note that the field names break the Golang naming convention, but it was done deliberately for JSON auto-parsing
// feature
type androidPaymentT struct {
    OrderId          string // nolint (for parsing json)
    PackageName      string
    ProductId        string // nolint (for parsing json)
    PurchaseTime     int64
    PurchaseState    uint8  // 0 (purchased), 1 (canceled), or 2 (refunded)
    PurchaseToken    string
    DeveloperPayload string
}

// @mitrakov (2017-04-18): don't use ALL_CAPS const naming (gometalinter, stackoverflow.com/questions/22688906)

// number of records for Top Ranking
const ratingCount = 10
// length of salt to store the passwords
const saltLen = 8
// length of promo code
const promocodeLen = 5
// standard reward for winner of Quick Battle, in gems
const rewardStd = 1
// time duration, after which an inactive user will be kicked out of the internal UserManager collection
const maxInactivityMin = 5
// time interval, when a UserManager periodically performs different tasks
const period = time.Minute

// ranking types
const (
    ratingGeneral = iota
    ratingWeekly
)

// UsrManager is an implementation of IUserManager.
// Both interface and implementation were placed in the same src intentionally!
// This component is independent.
type UsrManager struct {
    sync.RWMutex
    nameToUser    map[string]*User
    idToUser      map[uint64]*User
    sidToUser     map[Sid]*User
    usersTotal    map[uint64]bool           // only for statistics "Total users"
    sidManager    *TSidManager
    checker       *checker.SignatureChecker
    dbManager     IDbManager
    packer        IPacker
    controller    IController
    localArg      string
    skuGems       map[string]uint32
    ratingRewards map[int]uint32
    promoReward   uint32
    stop          chan bool
}

// NewUserManager creates a new instance of UsrManager and returns a reference to IUserManager interface.
// Please do not create a UsrManager directly.
// "sidManager" - reference to a TSidManager
// "checker" - reference to a SignatureChecker
// "dbManager" - reference to a IDbManager
// "packer" - reference to a IPacker
// "controller" - reference to a IController
// "localArg" - random host-specific string for generating password hashes
// "skuGems" - map [SKU -> price], e.g. "Map('gems_pack' -> 50)" means that gems_pack costs 50 gems
// "ratingRewards" - map [rating -> reward], e.g. "Map('rating.gold' -> 150)" means that a gold user will gain 150 gems
// "promoReward" - std reward for activating promo code (in gems)
func NewUserManager(sidManager *TSidManager, checker *checker.SignatureChecker, dbManager IDbManager, 
        packer IPacker, controller IController, localArg string, skuGems map[string]uint32, 
        ratingRewards map[int]uint32, promoReward uint32) IUserManager {
    Assert(sidManager, checker, dbManager, skuGems)

    usrMgr := new(UsrManager)
    usrMgr.nameToUser = make(map[string]*User)
    usrMgr.idToUser = make(map[uint64]*User)
    usrMgr.sidToUser = make(map[Sid]*User)
    usrMgr.usersTotal = make(map[uint64]bool)
    usrMgr.sidManager = sidManager
    usrMgr.checker = checker
    usrMgr.dbManager = dbManager
    usrMgr.packer = packer
    usrMgr.controller = controller
    usrMgr.localArg = localArg
    usrMgr.skuGems = skuGems
    usrMgr.ratingRewards = ratingRewards
    usrMgr.promoReward = promoReward
    usrMgr.stop = RunDaemon("user", period, func() {
        usrMgr.removeExpiredAbilities()
        usrMgr.kickOutInactiveUsers()
        usrMgr.updateWeekRating()
    })

    return usrMgr
}

// SetController assigns a non-NULL IController for this IUserManager
func (usrMgr *UsrManager) SetController(controller IController) {
    Assert(controller)
    usrMgr.controller = controller
}

// SignUp is a method to sign up a new user.
// important: all string lengths (name, email, etc.) are controlled by DBMS, so we don't need to check them manually
// early on, there were the following limits: name - 32 chars, other fields - 64 chars.
// "name" - user name
// "email" - user's e-mail
// "password" - user's password (NOT hash!)
// "agentInfo" - agent info (language, client version, OS, Android version, etc.)
// "promocode" - user's promo code
func (usrMgr *UsrManager) SignUp(name, email, password, agentInfo, promocode string) (user *User, err *Error) {
    Assert(usrMgr.dbManager)

    salt := RandString(saltLen)
    passwordFull := name + password + usrMgr.localArg
    hash := GetHash(passwordFull, salt)
    newPromocode := RandString(promocodeLen)
    err = usrMgr.dbManager.AddUser(name, email, hash, salt, newPromocode)
    if err == nil {
        user, err, _ = usrMgr.SignIn(name, password, agentInfo)
        if err == nil {
            if inviter, ok, err0 := usrMgr.IsPromocodeValid(promocode); ok {
                err1 := usrMgr.dbManager.ActivatePromocode(user.ID, inviter.ID)
                _, err2 := usrMgr.AddFriend(user, inviter.Name)
                _, err3 := usrMgr.AddFriend(inviter, user.Name)
                err = NewErrs(err0, err1, err2, err3)
            }
        }
    }
    return
}

// SignIn is a method to log in an existing user.
// "name" - user name
// "password" - user's password (NOT hash!)
// "agentInfo" - agent info (language, client version, OS, Android version, etc.)
func (usrMgr *UsrManager) SignIn(name, password, agentInfo string) (user *User, err *Error, oldSid Sid) {
    Assert(usrMgr.sidManager, usrMgr.dbManager)

    // if a user already exists, kick him/her out
    if oldUser, ok := usrMgr.GetUserByName(name); ok {
        oldSid = oldUser.Sid
        usrMgr.SignOut(oldUser)
    }

    // load a user from DB
    user, err = usrMgr.dbManager.GetUserByName(name)
    if err == nil {
        if user.AuthType == "Local" {
            passwordFull := name + password + usrMgr.localArg
            if CheckPassword(user.AuthData, passwordFull, user.Salt) { // password OK
                var sid Sid
                sid, err = usrMgr.sidManager.GetSid()
                if err == nil {
                    user.Sid = sid
                    user.AgentInfo = agentInfo
                    usrMgr.Lock()
                    usrMgr.nameToUser[user.Name] = user
                    usrMgr.idToUser[user.ID] = user
                    usrMgr.sidToUser[user.Sid] = user
                    usrMgr.usersTotal[user.ID] = true
                    usrMgr.Unlock()
                    go Check(usrMgr.dbManager.SetAgentInfo(user.ID, agentInfo))
                }
            } else {
                err = NewErr(usrMgr, 31, "Incorrect login/password")
            }
        } else {
            err = NewErr(usrMgr, 32, "Incorrect user auth type (%d)", user.AuthType)
        }
    }
    return
}

// SignOut is a method to log out a given user.
// This method is preferred, because it removes the user from internal collections and releases memory.
// Anyway if a client just silently disconnects without sighing out, then a user will be forcefully kicked out in
// several minutes by external mechanisms.
func (usrMgr *UsrManager) SignOut(user *User) {
    Assert(usrMgr.sidManager)

    usrMgr.Lock()
    delete(usrMgr.nameToUser, user.Name)
    delete(usrMgr.idToUser, user.ID)
    delete(usrMgr.sidToUser, user.Sid)
    // delete(usrMgr.usersTotal, user.ID)     don't delete from here for statistics purposes
    usrMgr.Unlock()
    usrMgr.sidManager.FreeSid(user.Sid)
}

// GetUserByName returns a user by name
func (usrMgr *UsrManager) GetUserByName(name string) (user *User, ok bool) {
    usrMgr.RLock()
    user, ok = usrMgr.nameToUser[name]
    usrMgr.RUnlock()
    if ok {
        user.setLastActiveNow()
    }
    return
}

// GetUserByID returns a user by ID (DB primary key)
func (usrMgr *UsrManager) GetUserByID(id uint64) (user *User, ok bool) {
    usrMgr.RLock()
    user, ok = usrMgr.idToUser[id]
    usrMgr.RUnlock()
    if ok {
        user.setLastActiveNow()
    }
    return
}

// GetUserBySid returns a user by Session ID
func (usrMgr *UsrManager) GetUserBySid(sid Sid) (user *User, ok bool) {
    usrMgr.RLock()
    user, ok = usrMgr.sidToUser[sid]
    usrMgr.RUnlock()
    if ok {
        user.setLastActiveNow()
    }
    return
}

// GetUserInfo returns info about a given user, expressed as a bytearray.
// The format is the following (all numbers are big-endian):
// - name (null-terminated string)
// - promo code (null-terminated string)
// - character (1 byte)
// - gems count (4 bytes)
// - abilities count (1 byte)
// - list of abilities, 3 bytes each (1 byte for ID and 2 bytes for time left for ability)
func (usrMgr *UsrManager) GetUserInfo(user *User) ([]byte, *Error) {
    Assert(user, usrMgr.dbManager)

    c1 := byte(user.Gems >> 24)
    c2 := byte(user.Gems >> 16)
    c3 := byte(user.Gems >> 8)
    c4 := byte(user.Gems)
    abilities, expires, err := usrMgr.dbManager.GetAbilities(user.ID)
    n := Min(uint(len(abilities)), uint(len(expires)))

    res := []byte(user.Name)
    res = append(res, 0) // NULL-terminator for name
    res = append(res, user.Promocode...)
    res = append(res, 0) // NULL-terminator for promocode
    res = append(res, user.Character, c1, c2, c3, c4, byte(n))
    for i := uint(0); i < n; i++ {
        minutes := expires[i].Sub(time.Now()).Minutes()
        if minutes < 0 { // if DeleteExpiredAbilities() is called not frequently enough, then value might be < 0
            minutes = 0
        } else if minutes > 65535 {
            minutes = 65535
        }
        minutesH := byte(uint16(minutes) / 256)
        minutesL := byte(uint16(minutes) % 256)
        res = append(res, abilities[i], minutesH, minutesL)
    }
    return res, err
}

// GetUserFriends returns the friend list (list of characters and list of names) of a given user.
// It is guaranteed that sizes of returned lists are equal
func (usrMgr *UsrManager) GetUserFriends(user *User) ([]byte, []string, *Error) {
    Assert(usrMgr.dbManager, user)
    return usrMgr.dbManager.GetUserFriends(user.ID)
}

// AddFriend adds a new friend for a given user.
// Note that this action affects DB as well.
func (usrMgr *UsrManager) AddFriend(user *User, name string) (character byte, err *Error) {
    Assert(usrMgr.dbManager, user)
    return usrMgr.dbManager.AddFriend(user.ID, name)
}

// RemoveFriend removes a friend (by a given name) for a given user.
// Note that this action affects DB as well.
func (usrMgr *UsrManager) RemoveFriend(user *User, name string) *Error {
    Assert(usrMgr.dbManager, user)
    return usrMgr.dbManager.RemoveFriend(user.ID, name)
}

// Accept registers a new battle of given users. For this specific method the order of them doesn't matter.
// Note that this action affects DB as well.
func (usrMgr *UsrManager) Accept(user1, user2 *User) *Error {
    Assert(usrMgr.dbManager)

    user1.LastEnemy = user2.ID
    user2.LastEnemy = user1.ID
    err1 := usrMgr.dbManager.SetLastEnemy(user1.ID, user2.ID)
    err2 := usrMgr.dbManager.SetLastEnemy(user2.ID, user1.ID)
    return NewErrs(err1, err2)
}

// GetUserAbilities returns the abilities (list of IDs) of a given user.
func (usrMgr *UsrManager) GetUserAbilities(user *User) ([]byte, *Error) {
    Assert(user, usrMgr.dbManager)
    ids, _, err := usrMgr.dbManager.GetAbilities(user.ID)
    return ids, err
}

// RewardUsers registers the end of the battle in DB, rewards the winner with some gems,
// adds trust points if necessary, rolls forward promo code bonuses if necessary, etc.
// Note that this action affects DB.
// "winnerSid" - Session ID of the winner
// "loserSid" - Session ID of the loser
// "score1" - score of user 1 (doesn't matter winner or loser)
// "score2" - score of user 2 (doesn't matter winner or loser)
// "trust" - trusted battle (by default Quick Battle is considered as 'trusted', and AI-mode is not)
// "box" - mail box
// nolint: gocyclo
func (usrMgr *UsrManager) RewardUsers(winnerSid, loserSid Sid, score1, score2 byte, trust bool, 
    box *MailBox) (reward uint32, err *Error) {

    Assert(usrMgr.dbManager)
    var err1, err2, err3, err4, err5, err6, err7, err8, err9 *Error

    // 1. Register rating
    err1 = usrMgr.registerRating(winnerSid, loserSid, score1, score2)
    // 2. Reward winner
    if user, ok := usrMgr.GetUserBySid(winnerSid); ok {
        if trust {
            // 2.1 Quick battle => add gems and ADD trust points
            if err2 = usrMgr.dbManager.RewardUser(user.ID, rewardStd, rewardStd); err2 == nil {
                reward = rewardStd
                user.Gems += rewardStd
                user.TrustPoints += rewardStd
            }
        } else {
            // 2.2. PvP battle => optionally add gems and CONSUME trust points
            if loser, ok := usrMgr.GetUserBySid(loserSid); ok {
                if loser.TrustPoints >= rewardStd {
                    if err3 = usrMgr.dbManager.ConsumeTrustPoints(loser.ID, rewardStd); err3 == nil {
                        loser.TrustPoints -= rewardStd
                    }
                    if err4 = usrMgr.dbManager.RewardUser(user.ID, rewardStd, 0 /*ZERO!*/); err4 == nil {
                        reward = rewardStd
                        user.Gems += rewardStd
                    }
                }
            }
        }
        // 3. Promocode (if a user has a promo code => both user and inviter get extra bonus)
        if score1 == 3 || score2 == 3 { // to avoid rewarding after tutorial/training levels
            if inviterID, ok, _ := usrMgr.dbManager.PromocodeExists(user.ID); ok {
                err5 = usrMgr.dbManager.DeactivatePromocode(user.ID, inviterID)
                err6 = usrMgr.dbManager.RewardUser(user.ID, usrMgr.promoReward, 0)
                err7 = usrMgr.dbManager.RewardUser(inviterID, usrMgr.promoReward, 0)
                if err6 == nil && err7 == nil {
                    user.Gems += usrMgr.promoReward
                    // if inviter is online => send a msg to both
                    if inviter, ok := usrMgr.GetUserByID(inviterID); ok {
                        inviter.Gems += usrMgr.promoReward
                        box.Put(winnerSid, usrMgr.packer.PackPromocodeDone(false, inviter.Name, usrMgr.promoReward))
                        box.Put(inviter.Sid, usrMgr.packer.PackPromocodeDone(true, user.Name, usrMgr.promoReward))
                        info, _ := usrMgr.GetUserInfo(inviter)
                        box.Put(inviter.Sid, usrMgr.packer.PackUserInfo(info))
                    } else { // inviter signed out => find his/her name in DB and send a msg only to a user
                        if inviter, err8 = usrMgr.dbManager.GetUserByID(inviterID); err8 == nil {
                            box.Put(winnerSid, usrMgr.packer.PackPromocodeDone(false, inviter.Name, usrMgr.promoReward))
                        }
                    }
                }
            }
        }
        // 4. Add user info message to the mailbox
        var info []byte
        info, err9 = usrMgr.GetUserInfo(user)
        box.Put(winnerSid, usrMgr.packer.PackUserInfo(info))
    } // else NOT an error (it just might be AI)
    
    err = NewErrs(err1, err2, err3, err4, err5, err6, err7, err9)
    return
}

// ChangeCharacter alters the "character" of a given user.
// Note that this action affects DB as well.
func (usrMgr *UsrManager) ChangeCharacter(user *User, character byte) *Error {
    Assert(usrMgr.dbManager, user)
    err := usrMgr.dbManager.ChangeUser(user.ID, user.Email, user.AuthData, character)
    if err == nil {
        user.Character = character
    }
    return err
}

// ChangePassword replaces a user's old password with a new one.
// Note that this action affects DB as well.
// "user" - user
// "oldPassword" - old password (NOT hash)
// "newPassword" - new password (NOT hash)
func (usrMgr *UsrManager) ChangePassword(user *User, oldPassword, newPassword string) *Error {
    Assert(usrMgr.dbManager, user)
    
    passwordFull := user.Name + oldPassword + usrMgr.localArg
    if CheckPassword(user.AuthData, passwordFull, user.Salt) { // password OK
        passwordFull = user.Name + newPassword + usrMgr.localArg
        hash := GetHash(passwordFull, user.Salt)
        err := usrMgr.dbManager.ChangeUser(user.ID, user.Email, hash, user.Character)
        if err == nil {
            user.AuthData = hash
        }
        return err
    }
    
    return NewErr(usrMgr, 33, "Cannot change password for", user.Name, "Old password incorrect")
}

// GetAllAbilities returns a full list of abilities, present in DB
func (usrMgr *UsrManager) GetAllAbilities() ([]byte, *Error) {
    Assert(usrMgr.dbManager)
    return usrMgr.dbManager.GetAllAbilities() // @mitrakov: possible bottleneck; maybe use cache?
}

// BuyProduct initiates purchasing a product (e.g. "Snorkel" for 3 days) for a given user
// "user" - user
// "code" - product code
// "days" - product duration (please note that it is not arbitrary value, the "days" must be present in DB)
func (usrMgr *UsrManager) BuyProduct(user *User, code, days byte) *Error {
    Assert(user, usrMgr.dbManager)
    cost, err := usrMgr.dbManager.BuyProduct(user.ID, code, days)
    if err == nil {
        user.Gems -= cost
    }
    return err
}

// GetWins returns total count of wins for a given user from DB
// @deprecated: not used since 1.3.8
func (usrMgr *UsrManager) GetWins(user *User) (uint32, *Error) {
    Assert(user, usrMgr.dbManager)
    return usrMgr.dbManager.GetWins(user.ID)
}

// GetRating returns "Top N" ranking by "ratingType" for a given user.
// "ratingType" - rating type (ratingGeneral, ratingWeekly)
// The format of a single ranking row is the following (all numbers are big-endian):
// - name (null-terminated string)
// - wins (4 bytes)
// - losses (4 bytes)
// - score difference (4 bytes)
func (usrMgr *UsrManager) GetRating(user *User, ratingType byte) ([]byte, *Error) {
    Assert(user, usrMgr.dbManager)
    return usrMgr.dbManager.GetRating(user.ID, ratingType+1, ratingCount) // +1 because DB needs values [1,2]
}

// IsPromocodeValid checks if a given "promocode" is valid.
// Aside from boolean value (valid/invalid), also returns the inviter User
func (usrMgr *UsrManager) IsPromocodeValid(promocode string) (inviter *User, ok bool, err *Error) {
    Assert(usrMgr.dbManager)

    if len(promocode) >= promocodeLen {
        name := promocode[0 : len(promocode)-promocodeLen]
        promo := promocode[len(promocode)-promocodeLen:]
        if inviter, err = usrMgr.dbManager.GetUserByName(name); err == nil {
            banned := strings.Contains(inviter.Promocode, "-")
            ok = inviter.Promocode == promo && !banned
        }
    }
    return
}

// GetUsersCount returns current count of users, stored in an internal collection
func (usrMgr *UsrManager) GetUsersCount() uint {
    usrMgr.RLock()
    defer usrMgr.RUnlock()
    return uint(len(usrMgr.nameToUser)) // len(map) is thread-safe but Data Race may occur
}

// GetUsersCountTotal returns total count of users for the whole Server lifetime
func (usrMgr *UsrManager) GetUsersCountTotal() uint {
    usrMgr.RLock()
    defer usrMgr.RUnlock()
    return uint(len(usrMgr.usersTotal))
}

// GetUserNumberN returns a user by a given sequence number in DB
func (usrMgr *UsrManager) GetUserNumberN(n uint) (*User, *Error) {
    Assert(usrMgr.dbManager)
    return usrMgr.dbManager.GetUserByNumber(n)
}

// GetSkuGems returns a map of all available SKUs -> price in gems, e.g. ("gems_pack" -> 50)
func (usrMgr *UsrManager) GetSkuGems() map[string]uint32 {
    Assert(usrMgr.skuGems)
    return usrMgr.skuGems
}

// CheckPayment verifies payment of a given user, by checking the signature of "jsonStr".
// "user" - user
// "jsonStr" - JSON that contains details about the purchase order
// "signature" - String containing the signature of the purchase data that was signed with the private key of the
// developer. The data signature uses the RSASSA-PKCS1-v1_5 scheme
func (usrMgr *UsrManager) CheckPayment(user *User, jsonStr, signature string) (gems uint32, box *MailBox, err *Error) {
    Assert(user, usrMgr.dbManager, usrMgr.checker, usrMgr.skuGems)
    
    var payment androidPaymentT
    er := json.Unmarshal([]byte(jsonStr), &payment)
    if er == nil {
        // for safety reasons, if our JSON contains username, we take that user instead of a current one
        // @mitrakov (2017-07-14): DeveloperPayload is ALWAYS EMPTY (see stackoverflow.com/questions/44756429 and my 
        // issue on the developers' website: github.com/googlesamples/android-play-billing/issues/67)
        if u, ok := usrMgr.GetUserByName(payment.DeveloperPayload); ok {
            user = u
        }
        err = usrMgr.dbManager.AddPayment(user.ID, payment.OrderId, payment.ProductId, payment.PurchaseTime, 
            payment.PurchaseToken, payment.PurchaseState)
        if err == nil {
            err = usrMgr.checker.CheckSignature(jsonStr, signature)
            if err == nil {
                err = usrMgr.dbManager.SetPaymentChecked(payment.OrderId)
                if err == nil {
                    var ok bool
                    if gems, ok = usrMgr.skuGems[payment.ProductId]; ok {
                        err = usrMgr.dbManager.RewardUser(user.ID, gems, gems)
                        if err == nil {
                            user.Gems += gems
                            user.TrustPoints += gems
                            err1 := usrMgr.dbManager.SetPaymentResult(payment.OrderId, gems)
                            info, err2 := usrMgr.GetUserInfo(user)
                            err = NewErrs(err1, err2)
                            box = NewMailBox()
                            box.Put(user.Sid, usrMgr.packer.PackUserInfo(info))
                        }
                    } else {
                        err = NewErr(usrMgr, 36, "Gems not found for sku: ", payment.ProductId)
                    }
                }
            }
        }
    } else {
        err = NewErrFromError(usrMgr, 37, er)
    }
    return
}

// Close shuts IUserManager down and releases all seized resources
func (usrMgr *UsrManager) Close() {
    Assert(usrMgr.stop)
    usrMgr.stop <- true
}

// ===============================
// === NON-INTERFACE FUNCTIONS ===
// ===============================

// removeExpiredAbilities removes expired abilities from ability lists of ALL users.
// This action doesn't affect users who are already in a battle.
// Note that this action affects DB as well.
// This method may be polled periodically.
func (usrMgr *UsrManager) removeExpiredAbilities() {
    Assert(usrMgr.dbManager, usrMgr.controller)

    removedIds, err := usrMgr.dbManager.DeleteExpiredAbilities()
    box := NewMailBox()
    for _, userID := range removedIds {
        if user, ok := usrMgr.GetUserByID(userID); ok {
            var info []byte
            info, err = usrMgr.GetUserInfo(user)
            box.Put(user.Sid, usrMgr.packer.PackUserInfo(info))
        } // else the user just signed out
    }
    usrMgr.controller.Event(box, err)
}

// updateWeekRating updates weekly ranking and rewards TOP-3 users of the week.
// This method may be polled periodically any time, but actually the ranking will be processed only once a week, on
// Mondays in 11:00 local time. In any other time this method will do nothing.
// Note, that Week Ranking will be cleared once TOP-3 users get awarded, and this action will affect DB as well.
func (usrMgr *UsrManager) updateWeekRating() {
    Assert(usrMgr.dbManager, usrMgr.controller, usrMgr.ratingRewards)

    t := time.Now()
    seconds := t.Minute()*60 + t.Second()
    monday1100 := t.Weekday() == 1 && t.Hour() == 11 && (seconds < int(period.Seconds()))
    if monday1100 {
        box := NewMailBox()
        users, err := usrMgr.dbManager.GetBestUsers(ratingWeekly+1, byte(len(usrMgr.ratingRewards)))
        if err == nil {
            for i, userID := range users {
                if reward, ok := usrMgr.ratingRewards[i]; ok {
                    err = usrMgr.dbManager.RewardUser(userID, reward, reward)
                    if user, ok := usrMgr.GetUserByID(userID); ok {
                        user.Gems += reward
                        user.TrustPoints += reward
                        info, err2 := usrMgr.GetUserInfo(user)
                        Check(err2)
                        box.Put(user.Sid, usrMgr.packer.PackUserInfo(info))
                    }
                } else {
                    err = NewErr(usrMgr, 38, "Reward index not found (%d)", i)
                }
                Check(err)
            }
            
            err = usrMgr.dbManager.ClearRating(ratingWeekly + 1)
        }
        usrMgr.controller.Event(box, err)
    }
}

// kickOutInactiveUsers kicks inactive users out after "maxInactivityMin" minutes of idleness.
// This method may be polled periodically.
func (usrMgr *UsrManager) kickOutInactiveUsers() {
    usrMgr.RLock()
    for _, user := range usrMgr.sidToUser {
        usrMgr.RUnlock()
        if time.Since(user.getLastActive()) > maxInactivityMin*time.Minute {
            usrMgr.SignOut(user) // this affects our map, but in Go it's safe (stackoverflow.com/questions/23229975)
            // here might be "sender.Send('you kicked')", but it's irrational (in 99% a user is just out of network)
        }
        usrMgr.RLock()
    }
    usrMgr.RUnlock()
}

// registerRating registers new battle result in DB for rankings.
// "winnerSid" - winner Session ID
// "loserSid" - loser Session ID
// "score1" - score of participant 1
// "score2" - score of participant 2
func (usrMgr *UsrManager) registerRating(winnerSid, loserSid Sid, score1, score2 byte) *Error {
    var err1, err2, err3, err4 *Error
    diff := Ternary(score1 >= score2, score1-score2, score2-score1) // math.Abs takes only float64

    if user, ok := usrMgr.GetUserBySid(winnerSid); ok {
        err1 = usrMgr.dbManager.RegisterWin(ratingGeneral+1, user.ID, diff) // +1 because DB needs values [1,2]
        err2 = usrMgr.dbManager.RegisterWin(ratingWeekly+1, user.ID, diff)  // +1 because DB needs values [1,2]
    } // else not an error (it might be AI)
    if loser, ok := usrMgr.GetUserBySid(loserSid); ok {
        err3 = usrMgr.dbManager.RegisterLoss(ratingGeneral+1, loser.ID, diff) // +1 because DB needs values [1,2]
        err4 = usrMgr.dbManager.RegisterLoss(ratingWeekly+1, loser.ID, diff)  // +1 because DB needs values [1,2]
    } // else not an error (it might be AI)
    return NewErrs(err1, err2, err3, err4)
}
