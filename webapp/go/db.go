package main

import (
	"database/sql"
	"errors"
	"github.com/gomodule/redigo/redis"
	"net/http"
	"strconv"
	"time"
)

var (
	ErrBannedIP      = errors.New("Banned IP")
	ErrLockedUser    = errors.New("Locked user")
	ErrUserNotFound  = errors.New("Not found user")
	ErrWrongPassword = errors.New("Wrong password")
)

const redisUserKey = "userId-"
const redisIpKey = "ip-"

func createLoginLog(succeeded bool, remoteAddr, login string, user *User) error {
	succ := 0
	c := redisConnection()
	defer c.Close()
	key := redisUserKey + strconv.Itoa(user.ID)
	ipKey := redisIpKey + remoteAddr
	if succeeded {
		succ = 1

		redisDel(key, c)
		redisDel(ipKey, c)

		//db.Exec(
		//	"UPDATE user SET failure_time = 0  WHERE id = ?",
		//	user.ID)
	}

	var userId sql.NullInt64
	if user != nil {
		userId.Int64 = int64(user.ID)
		userId.Valid = true
		if succ == 0 {
			failureTime := redisGetInt(key, c)
			redisSetInt(key, failureTime+1, c)
		}
		//db.Exec(
		//	"UPDATE user SET failure_time = failure_time + 1  WHERE id = ?",
		//	user.ID)
	}

	if succ == 0 {
		failureTime := redisGetInt(ipKey, c)
		redisSetInt(ipKey, failureTime+1, c)
	}

	_, err := db.Exec(
		"INSERT INTO login_log (`created_at`, `user_id`, `login`, `ip`, `succeeded`) "+
			"VALUES (?,?,?,?,?)",
		time.Now(), userId, login, remoteAddr, succ,
	)

	return err
}

func isLockedUser(user *User) (bool, error) {
	if user == nil {
		return false, nil
	}

	//var ni sql.NullInt64
	//row := db.QueryRow(
	//	"SELECT COUNT(1) AS failures FROM login_log WHERE "+
	//		"user_id = ? AND id > IFNULL((select id from login_log where user_id = ? AND "+
	//		"succeeded = 1 ORDER BY id DESC LIMIT 1), 0);",
	//	user.ID, user.ID,
	//)
	//
	//
	//err := row.Scan(&ni)
	//
	//switch {
	//case err == sql.ErrNoRows:
	//	return false, nil
	//case err != nil:
	//	return false, err
	//}
	//
	//var ni sql.NullInt64
	//row := db.QueryRow(
	//	"SELECT failure_time FROM users WHERE id = ?",
	//	user.ID,
	//)
	//
	//row.Scan(&ni)

	c := redisConnection()
	key := redisUserKey + strconv.Itoa(user.ID)
	failureTime := redisGetInt(key, c)

	return UserLockThreshold <= failureTime, nil
}

func isBannedIP(ip string) (bool, error) {
	//var ni sql.NullInt64
	//	//row := db.QueryRow(
	//	//	"SELECT COUNT(1) AS failures FROM login_log WHERE "+
	//	//		"ip = ? AND id > IFNULL((select id from login_log where ip = ? AND "+
	//	//		"succeeded = 1 ORDER BY id DESC LIMIT 1), 0);",
	//	//	ip, ip,
	//	//)
	//	//err := row.Scan(&ni)
	//	//
	//	//switch {
	//	//case err == sql.ErrNoRows:
	//	//	return false, nil
	//	//case err != nil:
	//	//	return false, err
	//	//}

	c := redisConnection()
	key := redisIpKey + ip
	failureTime := redisGetInt(key, c)

	return IPBanThreshold <= failureTime, nil
}

func attemptLogin(req *http.Request) (*User, error) {
	succeeded := false
	user := &User{}

	loginName := req.PostFormValue("login")
	password := req.PostFormValue("password")

	remoteAddr := req.RemoteAddr
	if xForwardedFor := req.Header.Get("X-Forwarded-For"); len(xForwardedFor) > 0 {
		remoteAddr = xForwardedFor
	}

	defer func() {
		createLoginLog(succeeded, remoteAddr, loginName, user)
	}()

	row := db.QueryRow(
		"SELECT id, login, password_hash, salt FROM users WHERE login = ?",
		loginName,
	)
	err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Salt)

	switch {
	case err == sql.ErrNoRows:
		user = nil
	case err != nil:
		return nil, err
	}

	if banned, _ := isBannedIP(remoteAddr); banned {
		return nil, ErrBannedIP
	}

	if locked, _ := isLockedUser(user); locked {
		return nil, ErrLockedUser
	}

	if user == nil {
		return nil, ErrUserNotFound
	}

	if user.PasswordHash != calcPassHash(password, user.Salt) {
		return nil, ErrWrongPassword
	}

	succeeded = true
	return user, nil
}

func getCurrentUser(userId interface{}) *User {
	user := &User{}
	row := db.QueryRow(
		"SELECT id, login, password_hash, salt FROM users WHERE id = ?",
		userId,
	)
	err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Salt)

	if err != nil {
		return nil
	}

	return user
}

func bannedIPs() []string {
	ips := []string{}

	rows, err := db.Query(
		"SELECT ip FROM "+
			"(SELECT ip, MAX(succeeded) as max_succeeded, COUNT(1) as cnt FROM login_log GROUP BY ip) "+
			"AS t0 WHERE t0.max_succeeded = 0 AND t0.cnt >= ?",
		IPBanThreshold,
	)

	if err != nil {
		return ips
	}

	defer rows.Close()
	for rows.Next() {
		var ip string

		if err := rows.Scan(&ip); err != nil {
			return ips
		}
		ips = append(ips, ip)
	}
	if err := rows.Err(); err != nil {
		return ips
	}

	rowsB, err := db.Query(
		"SELECT ip, MAX(id) AS last_login_id FROM login_log WHERE succeeded = 1 GROUP by ip",
	)

	if err != nil {
		return ips
	}

	defer rowsB.Close()
	for rowsB.Next() {
		var ip string
		var lastLoginId int

		if err := rows.Scan(&ip, &lastLoginId); err != nil {
			return ips
		}

		var count int

		err = db.QueryRow(
			"SELECT COUNT(1) AS cnt FROM login_log WHERE ip = ? AND ? < id",
			ip, lastLoginId,
		).Scan(&count)

		if err != nil {
			return ips
		}

		if IPBanThreshold <= count {
			ips = append(ips, ip)
		}
	}
	if err := rowsB.Err(); err != nil {
		return ips
	}

	return ips
}

func lockedUsers() []string {
	userIds := []string{}

	rows, err := db.Query(
		"SELECT user_id, login FROM "+
			"(SELECT user_id, login, MAX(succeeded) as max_succeeded, COUNT(1) as cnt FROM login_log GROUP BY user_id) "+
			"AS t0 WHERE t0.user_id IS NOT NULL AND t0.max_succeeded = 0 AND t0.cnt >= ?",
		UserLockThreshold,
	)

	if err != nil {
		return userIds
	}

	defer rows.Close()
	for rows.Next() {
		var userId int
		var login string

		if err := rows.Scan(&userId, &login); err != nil {
			return userIds
		}
		userIds = append(userIds, login)
	}
	if err := rows.Err(); err != nil {
		return userIds
	}

	rowsB, err := db.Query(
		"SELECT user_id, login, MAX(id) AS last_login_id FROM login_log WHERE user_id IS NOT NULL AND succeeded = 1 GROUP BY user_id",
	)

	if err != nil {
		return userIds
	}

	defer rowsB.Close()
	for rowsB.Next() {
		var userId int
		var login string
		var lastLoginId int

		if err := rowsB.Scan(&userId, &login, &lastLoginId); err != nil {
			return userIds
		}

		var count int

		err = db.QueryRow(
			"SELECT COUNT(1) AS cnt FROM login_log WHERE user_id = ? AND ? < id",
			userId, lastLoginId,
		).Scan(&count)

		if err != nil {
			return userIds
		}

		if UserLockThreshold <= count {
			userIds = append(userIds, login)
		}
	}
	if err := rowsB.Err(); err != nil {
		return userIds
	}

	return userIds
}

// redis接続
// 接続
//  c := redis_connection()
// defer c.Close()
//
//  var key = "KEY"
//  var val = "VALUE"
// redisSet(key, val, c)
// s := redisGet(key, c)
// fmt.Println(s)
func redisConnection() redis.Conn {
	const redisHost = "localhost:6379"

	//redisに接続
	c, err := redis.Dial("tcp", redisHost)
	if err != nil {
		panic(err)
	}

	return c
}

func redisSetInt(key string, value int, c redis.Conn) {
	c.Do("SET", key, value)
}

func redisDel(key string, c redis.Conn) {
	c.Do("DEL", key)
}

func redisGetInt(key string, c redis.Conn) int {
	s, err := redis.Int(c.Do("GET", key))
	if err != nil {
		return 0
	}

	return s
}

func redisLPush(key string, value string, c redis.Conn) {
	c.Do("LPUSH", key, value)
}

func redisLrange(key string, start int, end int, c redis.Conn) ([]string, error) {
	s, err := redis.Strings(c.Do("LRANGE", key, start, end))
	if err != nil {
		return nil, err
	}

	return s, nil
}
