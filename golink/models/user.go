package models

import (
    "crypto/md5"
    "database/sql"
    "errors"
    "fmt"
    "github.com/QLeelulu/goku"
    "github.com/QLeelulu/ohlala/golink/utils"
    "strings"
    "time"
)

type User struct {
    Id                   int64
    Name                 string
    Email                string
    Pwd                  string
    UserPic              string
    Description          string
    Permissions          int // 权限值，50以上是管理员，999是超级管理员
    ReferenceSystem      int
    ReferenceToken       string
    ReferenceTokenSecret string
    LinkCount            int
    FriendCount          int // 关注数量
    FollowerCount        int // 粉丝数量
    TopicCount           int // 分享的链接指定过的话题数量
    FtopicCount          int // 关注的话题数量
    Status               int // 用户状态: 正常、禁言、封号等等
    CreateTime           time.Time
}

func (u User) IsAdmin() bool {
    return u.Permissions >= 50
}

func (u User) GetGravatarUrl(size string) string {
    h := md5.New()
    h.Write([]byte(strings.ToLower(u.Email)))
    key := fmt.Sprintf("%x", h.Sum(nil))
    // default = "http://www.example.com/default.jpg"
    gravatarUrl := "http://www.gravatar.com/avatar/" + key + "?d=mm&s=" + size // d=default
    return gravatarUrl
}

type VUser struct {
    *User
    IsMe       bool // 是否登陆用户自己
    IsFollower bool // 是否粉丝
    IsFollowed bool // 是否已关注
    IsFriend   bool // 是否互相关注
}

// 转换为用于view的用户类型
func User_ToVUser(u *User, ctx *goku.HttpContext) *VUser {
    if u == nil {
        return nil
    }
    vu := &VUser{User: u}
    var userId int64
    if user, ok := ctx.Data["user"].(*User); ok && user != nil {
        userId = user.Id
    }
    if userId > 0 {
        if vu.Id == userId {
            vu.IsMe = true
        } else {
            vu.IsFollower, vu.IsFollowed, vu.IsFriend = User_CheckRelationship(userId, vu.Id)
        }
    }

    return vu
}

// 转换为用于view的用户类型
func User_ToVUsers(users []User, ctx *goku.HttpContext) []*VUser {
    if users == nil || len(users) < 1 {
        return nil
    }
    vusers := make([]*VUser, 0, len(users))
    for i, _ := range users {
        u := users[i]
        vusers = append(vusers, User_ToVUser(&u, ctx))
    }

    return vusers
}

// 检查 mUserId 与 sUserId 的关系，
// return: 
//      @isFollower: sUserId是否关注mUserId
//      @isFollowed: mUserId是否关注sUserId
//      @isFriend: 是否互相关注
func User_CheckRelationship(mUserId, sUserId int64) (isFollower, isFollowed, isFriend bool) {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    rows, err := db.Query("select * from `user_follow` where `user_id`=? and `follow_id`=? limit 1",
        mUserId, sUserId)
    if err != nil {
        goku.Logger().Errorln(err.Error())
        return
    }
    defer rows.Close()
    if rows.Next() {
        isFollowed = true
    }

    rows1, err1 := db.Query("select * from `user_follow` where `user_id`=? and `follow_id`=? limit 1",
        sUserId, mUserId)
    if err1 != nil {
        goku.Logger().Errorln(err1.Error())
        return
    }
    defer rows1.Close()
    if rows1.Next() {
        isFollower = true
    }

    if isFollowed && isFollower {
        isFriend = true
    }

    return
}

// 检查email地址是否存在。
// 任何出错都认为email地址存在，防止注册
func User_IsEmailExist(email string) bool {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    rows, err := db.Query("select id from `user` where `email_lower`=? limit 1", strings.ToLower(email))
    if err != nil {
        goku.Logger().Errorln(err.Error())
        // 出错直接认为email存在
        return true
    }
    defer rows.Close()
    if rows.Next() {
        return true
    }
    return false
}

func User_IsUserExist(name string) bool {
    user, _ := User_GetByName(name)
    if user != nil {
        return true
    }
    return false
}

// 检查账号密码是否正确
// 如果正确，则返回用户id
func User_CheckPwd(email, pwd string) int {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    pwd = utils.PasswordHash(pwd)
    rows, err := db.Query("select id from `user` where `email_lower`=? and pwd=? limit 1", strings.ToLower(email), pwd)
    if err != nil {
        goku.Logger().Errorln(err.Error())
        return 0
    }
    defer rows.Close()
    if rows.Next() {
        var id int
        err = rows.Scan(&id)
        if err != nil {
            goku.Logger().Errorln(err.Error())
        } else {
            return id
        }
    }
    return 0
}

func User_SaveMap(m map[string]interface{}) (sql.Result, error) {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()
    m["email_lower"] = strings.ToLower(m["email"].(string))
    m["name_lower"] = strings.ToLower(m["name"].(string))
    r, err := db.Insert("user", m)
    return r, err
}

func User_GetByTicket(ticket string) (*User, error) {
    redisClient := GetRedis()
    defer redisClient.Quit()

    id, err := redisClient.Get(ticket)
    if err != nil {
        return nil, err
    }

    if id.String() == "" {
        return nil, nil
    }

    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    var user *User = new(User)
    err = db.GetStruct(user, "id=?", id.String())
    if err != nil {
        return nil, err
    }
    if user.Id > 0 {
        return user, nil
    }
    return nil, nil
}

func User_GetById(id int64) *User {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    u := new(User)
    err := db.GetStruct(u, "id=?", id)
    if err != nil {
        goku.Logger().Errorln(err.Error())
    }
    if u.Id > 0 {
        return u
    }
    return nil
}

func User_GetByName(name string) (*User, error) {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    u := new(User)
    err := db.GetStruct(u, "name_lower=?", strings.ToLower(name))
    if err != nil {
        goku.Logger().Errorln(err.Error())
        return nil, err
    }
    if u.Id > 0 {
        return u, nil
    }
    return nil, nil
}

func User_Update(id int64, m map[string]interface{}) (sql.Result, error) {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()
    r, err := db.Update("user", m, "id=?", id)
    return r, err
}

// 删除用户
// 应该做成标记删除的方式
func User_Delete(id int) (sql.Result, error) {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()
    r, err := db.Delete("user", "id=?", id)
    return r, err
}

// userId 关注 followId
func User_Follow(userId, followId int64) (bool, error) {
    if userId < 1 || followId < 1 {
        return false, errors.New("参数错误")
    }
    if userId == followId {
        return false, errors.New("不能关注自己")
    }
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    vals := map[string]interface{}{
        "user_id":     userId,
        "follow_id":   followId,
        "create_time": time.Now(),
    }
    r, err := db.Insert("user_follow", vals)
    if err != nil {
        if strings.Index(err.Error(), "Duplicate entry") > -1 {
            return false, errors.New("已经关注该用户")
        } else {
            goku.Logger().Errorln(err.Error())
            return false, err
        }
    }

    var afrow int64
    afrow, err = r.RowsAffected()
    if err != nil {
        goku.Logger().Errorln(err.Error())
        return false, err
    }

    if afrow > 0 {
        LinkForUser_FollowUser(userId, followId)
        // 更新粉丝数
        User_IncCount(db, userId, "friend_count", 1)
        // 更新关注数
        User_IncCount(db, followId, "follower_count", 1)
        return true, nil
    }
    return false, nil
}

// userId 取消关注 followId
func User_UnFollow(userId, followId int64) (bool, error) {
    if userId < 1 || followId < 1 {
        return false, errors.New("参数错误")
    }
    if userId == followId {
        return false, errors.New("不能取消关注自己")
    }
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    r, err := db.Delete("user_follow", "`user_id`=? AND `follow_id`=?", userId, followId)
    if err != nil {
        goku.Logger().Errorln(err.Error())
        return false, err
    }

    var afrow int64
    afrow, err = r.RowsAffected()
    if err != nil {
        goku.Logger().Errorln(err.Error())
        return false, err
    }

    if afrow > 0 {
        LinkForUser_UnFollowUser(userId, followId)
        // 更新粉丝数
        User_IncCount(db, userId, "friend_count", -1)
        // 更新关注数
        User_IncCount(db, followId, "follower_count", -1)
        return true, nil
    }
    return false, nil
}

// 加（减）用户信息里面的统计数据
// @field: 要修改的字段
// @inc: 要增加或减少的值
func User_IncCount(db *goku.MysqlDB, userid int64, field string, inc int) (sql.Result, error) {
    // m := map[string]interface{}{field: fmt.Sprintf("%v+%v", field, inc)}
    // r, err := db.Update("user", m, "id=?", userid)
    r, err := db.Exec(fmt.Sprintf("UPDATE `user` SET %s=%s+? WHERE id=?;", field, field), inc, userid)
    if err != nil {
        goku.Logger().Errorln(err.Error())
    }
    return r, err
}

// 获取用户关注的话题列表
func User_GetFollowTopics(userId int64, page, pagesize int) ([]Topic, error) {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    page, pagesize = utils.PageCheck(page, pagesize)

    qi := goku.SqlQueryInfo{}
    qi.Fields = "t.id, t.name, t.description, t.pic"
    qi.Join = " tf INNER JOIN `topic` t ON tf.topic_id=t.id"
    qi.Where = "tf.user_id=?"
    qi.Params = []interface{}{userId}
    qi.Limit = pagesize
    qi.Offset = pagesize * page
    qi.Order = "t.id desc"

    rows, err := db.Select("topic_follow", qi)

    if err != nil {
        goku.Logger().Errorln(err.Error())
        return nil, err
    }
    defer rows.Close()

    topics := make([]Topic, 0)
    for rows.Next() {
        topic := Topic{}
        err = rows.Scan(&topic.Id, &topic.Name, &topic.Description, &topic.Pic)
        if err != nil {
            goku.Logger().Errorln(err.Error())
            return nil, err
        }
        topics = append(topics, topic)
    }
    return topics, nil
}

// 获取用户参与的话题（即用户发link时提及的话题）

// 获取用户列表.
// @page: 从1开始的页数
// @return: users, total-count, err
func User_GetList(page, pagesize int, order string) ([]User, int64, error) {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    page, pagesize = utils.PageCheck(page, pagesize)

    qi := goku.SqlQueryInfo{}
    qi.Limit = pagesize
    qi.Offset = pagesize * page
    if order == "" {
        qi.Order = "id desc"
    } else {
        qi.Order = order
    }

    var users []User
    err := db.GetStructs(&users, qi)
    if err != nil {
        goku.Logger().Errorln(err.Error())
        return nil, 0, err
    }

    total, err := db.Count("user", "")
    if err != nil {
        goku.Logger().Errorln(err.Error())
    }
    return users, total, nil
}

//创建关联系统的用户
func Exists_Reference_System_User(accesstoken string, uid string, reference_system int) (int64, string, error) {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    rows, err := db.Query("select id,email_lower from `user` where `reference_system`=? and `reference_id`=? limit 1", reference_system, uid)
    if err != nil {
        goku.Logger().Errorln(err.Error())
        return 0, "", err
    }

    if rows.Next() {
        var userId int64
        var email_lower string
        rows.Scan(&userId, email_lower)
        db.Query("UPDATE `user` SET reference_token=? where `id`=? limit 1", accesstoken, userId)

        return userId, email_lower, nil
    } else {
        return 0, "", nil
    }

    return 0, "", nil
}

//模糊搜索用户
func User_SearchByName(name string, ctx *goku.HttpContext) ([]*VUser, error) {
    var db *goku.MysqlDB = GetDB()
    defer db.Close()

    qi := goku.SqlQueryInfo{}
    qi.Fields = "`id`,`name`,`email`,`description`,`user_pic`,`friend_count`,`topic_count`,`ftopic_count`,`status`,`follower_count`,`link_count`,`create_time`"
    qi.Where = "name_lower LIKE ?"
    qi.Params = []interface{}{strings.ToLower(name) + "%"}
    qi.Limit = 10
    qi.Offset = 0
    qi.Order = "link_count DESC"

    rows, err := db.Select("user", qi)

    if err != nil {
        goku.Logger().Errorln(err.Error())
        return nil, err
    }

    users := make([]User, 0)
    for rows.Next() {
        user := User{}
        err = rows.Scan(&user.Id, &user.Name, &user.Email, &user.Description, &user.UserPic, &user.FriendCount, &user.TopicCount, &user.FtopicCount, &user.Status, &user.FollowerCount, &user.LinkCount, &user.CreateTime)
        if err != nil {
            goku.Logger().Errorln(err.Error())
            return nil, err
        }
        users = append(users, user)
    }

    return User_ToVUsers(users, ctx), nil

}

//根据用户关注的话题给它推荐相关的用户




