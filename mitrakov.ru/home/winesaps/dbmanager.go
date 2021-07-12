// Copyright 2017-2018 Artem Mitrakov. All rights reserved.
package main

import "fmt"
import "time"
import "database/sql"
import _ "github.com/go-sql-driver/mysql" // stackoverflow.com/questions/21220077
import "mitrakov.ru/home/winesaps/user"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// A DbManager is a component that operates with DBMS. Please note that this is THE ONLY component designed for those
// purposes. Feel free to expand this class with new methods
// This component is "dependent"
type DbManager struct {
    db *sql.DB
}

// NewDbManager creates a new instance of DbManager. Please do not create DbManager directly.
// "username" - DB username
// "pwd" - DB password
func NewDbManager(username, pwd string) (*DbManager, *Error) {
    // @mitrakov: 'parseTime=true' needed for scanning MySQL timestamps
    srcName := fmt.Sprintf("%s:%s@tcp(%s:%d)/rush?parseTime=true&charset=utf8", username, pwd, "mysql-service", 3306)
    db, err := sql.Open("mysql", srcName)
    if err == nil {
        return &DbManager{db}, nil
    }
    return nil, NewErrFromError("DbManager", 200, err)
}

// AddUser inserts a new user into DB
// "name" - user name
// "email" - user's e-mail
// "hash" - hash of user's password
// "salt" - salt
// "promocode" - user's promo code
func (dbMgr *DbManager) AddUser(name, email, hash, salt, promocode string) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("INSERT INTO user SET name=?, email=?, auth_data=?, salt=?, promocode=?")
    if err == nil {
        _, err = stmt.Exec(name, email, hash, salt, promocode)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 201, err)
}

// GetUserByID returns a user by ID
// "id" - user ID
func (dbMgr *DbManager) GetUserByID(id uint64) (*user.User, *Error) {
    sql := "SELECT user_id, name, email, auth_type, auth_data, salt, promocode, `character`+0," +
        " gems, trust_points, last_enemy, agent_info, last_login FROM user WHERE user_id=?"
    return dbMgr.getUserBySQL(sql, id)
}

// GetUserByName returns a user by name
// "name" - user name
func (dbMgr *DbManager) GetUserByName(name string) (*user.User, *Error) {
    sql := "SELECT user_id, name, email, auth_type, auth_data, salt, promocode, `character`+0," +
        " gems, trust_points, last_enemy, agent_info, last_login FROM user WHERE name=?"
    return dbMgr.getUserBySQL(sql, name)
}

// GetUserByNumber returns a user by order number according to how he/she is stored in DB
// "number" - user position in DB table
func (dbMgr *DbManager) GetUserByNumber(number uint) (*user.User, *Error) {
    sql := "SELECT user_id, name, email, auth_type, auth_data, salt, promocode, `character`+0," +
        " gems, trust_points, last_enemy, agent_info, last_login FROM user LIMIT ?, 1"
    return dbMgr.getUserBySQL(sql, number-1)
}

// GetAllAbilities return all possible abilities
func (dbMgr *DbManager) GetAllAbilities() ([]byte, *Error) {
    Assert(dbMgr.db)
    res := []byte{}
    rows, err := dbMgr.db.Query("SELECT name+0, days, gems FROM ability")
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var id, days, cost byte
            err = rows.Scan(&id, &days, &cost)
            if err == nil {
                res = append(res, id, days, cost)
            } else {
                return res, NewErrFromError(dbMgr, 206, err) // this return is necessary because it's in a loop
            }
        }
    }
    return res, NewErrFromError(dbMgr, 207, err)
}

// SetLastEnemy assigns last enemy (enemyID) to a given user (userID)
// "userID" - user ID
// "enemyID" - user's enemy ID
func (dbMgr *DbManager) SetLastEnemy(userID, enemyID uint64) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("UPDATE user SET last_enemy=? WHERE user_id=?")
    if err == nil {
        _, err = stmt.Exec(enemyID, userID)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 208, err)
}

// SetAgentInfo sets agent info (language, client version, OS, Android version, etc.) to a given user
// "userID" - user ID
// "agentInfo" - agent info
func (dbMgr *DbManager) SetAgentInfo(userID uint64, agentInfo string) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("UPDATE user SET agent_info=? WHERE user_id=?")
    if err == nil {
        _, err = stmt.Exec(agentInfo, userID)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 236, err)
}

// RegisterWin inserts a new battle result (WIN) to the Rankings table
// "ratingType" - rating type (IMPORTANT: one-based, not zero-based)
// "userID" - user ID
// "scoreDiff" - score difference (abs[score1-score2])
func (dbMgr *DbManager) RegisterWin(ratingType byte, userID uint64, scoreDiff byte) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("INSERT INTO rating (user_id, type, wins, losses, score_diff) " +
        "VALUES(?, ?, 1, 0, ?) ON DUPLICATE KEY UPDATE wins = wins + 1, score_diff = score_diff + ?")
    if err == nil {
        _, err = stmt.Exec(userID, ratingType, scoreDiff, scoreDiff)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 209, err)
}

// RegisterLoss inserts a new battle result (LOSS) to the Rankings table
// "ratingType" - rating type (IMPORTANT: one-based, not zero-based)
// "userID" - user ID
// "scoreDiff" - score difference (abs[score1-score2])
func (dbMgr *DbManager) RegisterLoss(ratingType byte, userID uint64, scoreDiff byte) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("INSERT INTO rating (user_id, type, wins, losses, score_diff) " +
        "VALUES(?, ?, 0, 1, ?) ON DUPLICATE KEY UPDATE losses = losses + 1, score_diff = score_diff + ?")
    if err == nil {
        negateDiff := -1 * int(scoreDiff) // don't use "-scoreDiff": it produces numbers like 255 instead of -1
        _, err = stmt.Exec(userID, ratingType, negateDiff, negateDiff)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 210, err)
}

// RewardUser gives a user some gems and trustPoints (see documentation to learn what "trustPoints" are)
// "userID" - user ID
// "gems" - reward, in gems
// "trustPoints" - trust points
func (dbMgr *DbManager) RewardUser(userID uint64, gems, trustPoints uint32) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("UPDATE user SET gems = gems+?, trust_points = trust_points+? WHERE user_id=?")
    if err == nil {
        _, err = stmt.Exec(gems, trustPoints, userID)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 211, err)
}

// ConsumeTrustPoints decrements "trustPoints" parameter of a user
// "userID" - user ID
// "trustPoints" - trust points to consume
func (dbMgr *DbManager) ConsumeTrustPoints(userID uint64, trustPoints uint32) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("UPDATE user SET trust_points = trust_points-? WHERE user_id=?")
    if err == nil {
        _, err = stmt.Exec(trustPoints, userID)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 235, err)
}

// ChangeUser updates the info about a user
// "userID" - user ID
// "email" - user e-mail
// "hash" - hash of user's password
// "character" - user's character
func (dbMgr *DbManager) ChangeUser(userID uint64, email, hash string, character byte) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("UPDATE user SET email = ?, auth_data = ?, `character` = ? WHERE user_id = ?")
    if err == nil {
        _, err = stmt.Exec(email, hash, character, userID)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 212, err)
}

// GetAbilities returns the abilities (list of IDs and list of expire timestamps) of a given user. It is guaranteed that
// sizes of returned lists are equal
// "userID" - user ID
func (dbMgr *DbManager) GetAbilities(userID uint64) ([]byte, []time.Time, *Error) {
    Assert(dbMgr.db)
    ids := []byte{}
    expires := []time.Time{}
    stmt, err := dbMgr.db.Prepare("SELECT name+0, expire FROM user_ability WHERE user_id=?")
    if err == nil {
        defer stmt.Close()
        var rows *sql.Rows
        rows, err = stmt.Query(userID)
        if err == nil {
            for rows.Next() {
                var id byte
                var expire time.Time
                err = rows.Scan(&id, &expire)
                if err == nil {
                    ids = append(ids, id)
                    expires = append(expires, Convert(expire))
                } else {
                    return ids, expires, NewErrFromError(dbMgr, 213, err) // this return is necessary because of a loop
                }
            }
        }
    }
    return ids, expires, NewErrFromError(dbMgr, 214, err)
}

// BuyProduct initiates purchasing a product (e.g. "Climbing Shoes" for 7 days) for a given user
// "userID" - user ID
// "code" - product code
// "days" - duration, in days (please note that it's not arbitrary value, the "days" must be present in "ability" table)
func (dbMgr *DbManager) BuyProduct(userID uint64, code, days byte) (cost uint32, error *Error) {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("SELECT sp_buy(?, ?, ?)")
    if err == nil {
        err = stmt.QueryRow(userID, code, days).Scan(&cost) // row is always != nil
       Check( stmt.Close())
    }
    return cost, NewErrFromError(dbMgr, 215, err)
}

// GetWins returns count of wins from the Ranking table for a given user
// @deprecated: not used since 1.3.8
// "userID" - user ID
func (dbMgr *DbManager) GetWins(userID uint64) (wins uint32, error *Error) {
    Assert(dbMgr.db)
    query := "SELECT IFNULL((SELECT wins FROM rating WHERE user_id = ? AND type = 'General'), 0) AS wins"
    stmt, err := dbMgr.db.Prepare(query)
    if err == nil {
        err = stmt.QueryRow(userID).Scan(&wins) // row is always != nil
        Check(stmt.Close())
    }
    return wins, NewErrFromError(dbMgr, 234, err)
}

// GetRating returns Ranking for a given user of a given ratingType (ratingGeneral or ratingWeekly). Please specify
// limit to avoid performance issues (default is 10)
// "userID" - user ID
// "ratingType" - rating type (IMPORTANT: one-based, not zero-based)
// "limit" - data sample limit
func (dbMgr *DbManager) GetRating(userID uint64, ratingType, limit byte) ([]byte, *Error) {
    Assert(dbMgr.db)
    res := []byte{}
    stmt, err := dbMgr.db.Prepare("(SELECT name, wins, losses, score_diff FROM rating JOIN user USING(user_id) " +
        "WHERE type = ? ORDER BY victory_diff DESC, score_diff DESC, wins DESC LIMIT ?) " +
        "UNION (SELECT name, wins, losses, score_diff FROM rating JOIN user USING(user_id) " +
        "WHERE user_id = ? AND type = ?)")
    if err == nil {
        defer stmt.Close()
        var rows *sql.Rows
        rows, err = stmt.Query(ratingType, limit, userID, ratingType)
        if err == nil {
            for rows.Next() {
                var name string
                var wins, losses uint32
                var scoreDiff int
                err = rows.Scan(&name, &wins, &losses, &scoreDiff)
                if err == nil {
                    w1 := byte(wins >> 24)
                    w2 := byte(wins >> 16)
                    w3 := byte(wins >> 8)
                    w4 := byte(wins)
                    l1 := byte(losses >> 24)
                    l2 := byte(losses >> 16)
                    l3 := byte(losses >> 8)
                    l4 := byte(losses)
                    s1 := byte(scoreDiff >> 24)
                    s2 := byte(scoreDiff >> 16)
                    s3 := byte(scoreDiff >> 8)
                    s4 := byte(scoreDiff)
                    res = append(res, []byte(name)...)
                    res = append(res, 0, w1, w2, w3, w4, l1, l2, l3, l4, s1, s2, s3, s4) // 0 is a terminating NULL
                } else {
                    return res, NewErrFromError(dbMgr, 216, err) // this return is necessary because it's in a loop
                }
            }
        }
    }
    return res, NewErrFromError(dbMgr, 217, err)
}

// GetBestUsers returns Top N Ranking of a given ratingType (ratingGeneral or ratingWeekly)
// "ratingType" - rating type (IMPORTANT: one-based, not zero-based)
// "limit" - data sample limit
func (dbMgr *DbManager) GetBestUsers(ratingType, limit byte) (ids []uint64, error *Error) {
    Assert(dbMgr.db)
    res := []uint64{}
    query := "SELECT user_id FROM rating WHERE type = ? ORDER BY victory_diff DESC, score_diff DESC, wins DESC LIMIT ?"
    stmt, err := dbMgr.db.Prepare(query)
    if err == nil {
        defer stmt.Close()
        var rows *sql.Rows
        rows, err = stmt.Query(ratingType, limit)
        if err == nil {
            for rows.Next() {
                var userID uint64
                err = rows.Scan(&userID)
                if err == nil {
                    res = append(res, userID)
                } else {
                    return res, NewErrFromError(dbMgr, 218, err) // this return is necessary because it's in a loop
                }
            }
        }
    }
    return res, NewErrFromError(dbMgr, 219, err)
}

// ClearRating erases Ranking table of a given ratingType (ratingGeneral or ratingWeekly)
// "ratingType" - rating type (IMPORTANT: one-based, not zero-based)
func (dbMgr *DbManager) ClearRating(ratingType byte) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("DELETE FROM rating WHERE type = ?")
    if err == nil {
        _, err = stmt.Exec(ratingType)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 220, err)
}

// GetUserFriends returns friends (list of characters and list of names) of a given user. It is guaranteed that sizes of
// returned lists are equal
// "userID" - user ID
func (dbMgr *DbManager) GetUserFriends(userID uint64) ([]byte, []string, *Error) {
    Assert(dbMgr.db)
    res0 := []byte{}
    res1 := []string{}
    stmt, err := dbMgr.db.Prepare("SELECT `character`+0, name FROM friend JOIN user " +
        "ON friend.friend_user_id = user.user_id WHERE friend.user_id = ?")
    if err == nil {
        defer stmt.Close()
        var rows *sql.Rows
        rows, err = stmt.Query(userID)
        if err == nil {
            for rows.Next() {
                var character byte
                var name string
                err = rows.Scan(&character, &name)
                if err == nil {
                    res0 = append(res0, character)
                    res1 = append(res1, name)
                } else {
                    return res0, res1, NewErrFromError(dbMgr, 221, err) // return is necessary because it's in a loop
                }
            }
        }
    }

    return res0, res1, NewErrFromError(dbMgr, 222, err)
}

// AddFriend inserts a new friend for a given user
// "userID" - user ID
// "name" - friend's name
func (dbMgr *DbManager) AddFriend(userID uint64, name string) (character byte, e *Error) {
    Assert(dbMgr.db)
    sql := "INSERT INTO friend (user_id, friend_user_id) VALUES (?, (SELECT user_id FROM user WHERE name = ?))"
    stmt, err := dbMgr.db.Prepare(sql)
    if err == nil {
        _, err = stmt.Exec(userID, name)
        Check(stmt.Close())
        if err == nil {
            stmt, err = dbMgr.db.Prepare("SELECT `character`+0 FROM user WHERE name = ?")
            if err == nil {
                err = stmt.QueryRow(name).Scan(&character) // row is always != nil
                Check(stmt.Close())
            }
        }
    }
    e = NewErrFromError(dbMgr, 223, err)
    return
}

// RemoveFriend removes a friend (by the name) for a given user
// "userID" - user ID
// "name" - friend's name
func (dbMgr *DbManager) RemoveFriend(userID uint64, name string) *Error {
    Assert(dbMgr.db)
    sql := "DELETE FROM friend WHERE user_id = ? AND friend_user_id = (SELECT user_id FROM user WHERE name = ?)"
    stmt, err := dbMgr.db.Prepare(sql)
    if err == nil {
        _, err = stmt.Exec(userID, name)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 224, err)
}

// ActivatePromocode activates promocode of a user specified by inviterID for a user specified by userID
// "userID" - user ID
// "inviterID" - user ID of inviter
func (dbMgr *DbManager) ActivatePromocode(userID, inviterID uint64) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("INSERT INTO promocode (user_id, inviter_user_id) VALUES (?, ?)")
    if err == nil {
        _, err = stmt.Exec(userID, inviterID)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 225, err)
}

// DeactivatePromocode de-activates promocode of a user specified by inviterID for a user specified by userID
// "userID" - user ID
// "inviterID" - user ID of inviter
func (dbMgr *DbManager) DeactivatePromocode(userID, inviterID uint64) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("UPDATE promocode SET promo = 'Used' WHERE user_id = ? AND inviter_user_id = ?")
    if err == nil {
        _, err = stmt.Exec(userID, inviterID)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 226, err)
}

// PromocodeExists checks whether promocode exists for a given userID
// "userID" - user ID
func (dbMgr *DbManager) PromocodeExists(userID uint64) (inviterID uint64, exists bool, err *Error) {
    Assert(dbMgr.db)
    stmt, er := dbMgr.db.Prepare("SELECT inviter_user_id FROM promocode WHERE user_id = ? AND promo = 'Pending'")
    err = NewErrFromError(dbMgr, 227, er)
    if err == nil {
        exists = stmt.QueryRow(userID).Scan(&inviterID) != sql.ErrNoRows // row is always != nil
        Check(stmt.Close())
    }
    return
}

// DeleteExpiredAbilities removes expired abilities. It is designed to be executed periodically
func (dbMgr *DbManager) DeleteExpiredAbilities() (removedIds []uint64, error *Error) {
    Assert(dbMgr.db)
    res := []uint64{}
    rows, err := dbMgr.db.Query("SELECT DISTINCT(user_id) FROM user_ability WHERE expire < CURRENT_TIMESTAMP")
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var id uint64
            err = rows.Scan(&id)
            if err == nil {
                res = append(res, id)
            } else {
                return res, NewErrFromError(dbMgr, 228, err) // this return is necessary because it's in a loop
            }
        }
        _, err = dbMgr.db.Exec("DELETE FROM user_ability WHERE expire < CURRENT_TIMESTAMP")
    }

    return res, NewErrFromError(dbMgr, 229, err)
}

// AddPayment inserts info about new purchase
// "userID" - user ID
// "orderID" - order ID (returned by a platform)
// "sku" - stock keeping unit
// "tsMsec" - timestamp of operation
// "data" - raw data
// "state" - status (0 = purchased, 1 = cancelled, 2 = refunded)
func (dbMgr *DbManager) AddPayment(userID uint64, orderID, sku string, tsMsec int64, data string, state uint8) *Error {
    Assert(dbMgr.db)
    sql := "INSERT INTO payment (user_id, order_id, sku, stamp, data, state) VALUES (?, ?, ?, ?, ?, ?)"
    stmt, err := dbMgr.db.Prepare(sql)
    if err == nil {
        t := time.Unix(tsMsec/1000, (tsMsec%1000)*1000) // convert to MySQL-compatible datetime
        _, err = stmt.Exec(userID, orderID, sku, t, data, state)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 230, err)
}

// SetPaymentChecked sets the payment, defined by orderID, as verified
// "orderID" - order ID (returned by a platform)
func (dbMgr *DbManager) SetPaymentChecked(orderID string) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("UPDATE payment SET checked = 1 WHERE order_id = ?")
    if err == nil {
        _, err = stmt.Exec(orderID)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 231, err)
}

// SetPaymentResult "finishes" payment transaction by adding gems for a successful purchase
// "orderID" - order ID (returned by a platform)
// "gems" - gems sold by the transaction
func (dbMgr *DbManager) SetPaymentResult(orderID string, gems uint32) *Error {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare("UPDATE payment SET gems = ? WHERE order_id = ?")
    if err == nil {
        _, err = stmt.Exec(gems, orderID)
        Check(stmt.Close())
    }
    return NewErrFromError(dbMgr, 232, err)
}

// Close shuts DB down and releases all seized resources
func (dbMgr *DbManager) Close() *Error {
    Assert(dbMgr.db)
    err := dbMgr.db.Close()
    return NewErrFromError(dbMgr, 233, err)
}

// ===============================
// ===    PRIVATE FUNCTIONS    ===
// ===============================

// getUserBySql returns a user by given SQL (for internal usage only!)
// "query" - sql query
// "arg" - sql argument
func (dbMgr *DbManager) getUserBySQL(query string, arg interface{}) (*user.User, *Error) {
    Assert(dbMgr.db)
    stmt, err := dbMgr.db.Prepare(query)
    if err == nil {
        defer stmt.Close()
        row := stmt.QueryRow(arg) // row is always != nil
        var userID uint64
        var name string
        var email string
        var authType string
        var authData string
        var salt string
        var promo string
        var character byte
        var gems uint32
        var tp uint32
        var lastEnemy sql.NullInt64
        var agentInfo string
        var lastLogin time.Time

        err = row.Scan(&userID, &name, &email, &authType, &authData, &salt, &promo, &character, &gems, &tp, &lastEnemy, 
            &agentInfo, &lastLogin)
        if err == nil {
            return &user.User{Character: character, Gems: gems, TrustPoints: tp, Name: name, Email: email, 
                AuthType: authType, AuthData: authData, Salt: salt, Promocode: promo, ID: userID,
                LastEnemy: uint64(lastEnemy.Int64), AgentInfo: agentInfo, LastLogin: Convert(lastLogin), 
                LastActive: time.Now()}, nil
        }
        return nil, NewErrFromError(dbMgr, 202, err)
    }
    return nil, NewErrFromError(dbMgr, 203, err)
}
