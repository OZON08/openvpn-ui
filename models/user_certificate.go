package models

import (
	"fmt"

	"github.com/beego/beego/v2/client/orm"
)

type UserCertificate struct {
	Id       int64  `orm:"auto"`
	UserId   int64  `orm:"index"`
	CertName string `orm:"size(128)"`
}

func CertsForUser(userID int64) ([]string, error) {
	var rows []UserCertificate
	_, err := orm.NewOrm().QueryTable(new(UserCertificate)).
		Filter("UserId", userID).All(&rows)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.CertName)
	}
	return names, nil
}

// AllAssignedCerts returns every cert name that is assigned to any user.
func AllAssignedCerts() ([]string, error) {
	var rows []UserCertificate
	if _, err := orm.NewOrm().QueryTable(new(UserCertificate)).All(&rows); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(rows))
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		if _, dup := seen[r.CertName]; !dup {
			seen[r.CertName] = struct{}{}
			names = append(names, r.CertName)
		}
	}
	return names, nil
}

func AssignCert(userID int64, certName string) error {
	o := orm.NewOrm()
	if o.QueryTable(new(UserCertificate)).Filter("CertName", certName).Exist() {
		return fmt.Errorf("certificate %q is already assigned to a user", certName)
	}
	_, err := o.Insert(&UserCertificate{UserId: userID, CertName: certName})
	return err
}

func RemoveCert(userID int64, certName string) error {
	_, err := orm.NewOrm().QueryTable(new(UserCertificate)).
		Filter("UserId", userID).Filter("CertName", certName).Delete()
	return err
}

// TransferCert atomically removes a cert assignment from one user and assigns it
// to another. If toUserID already has the assignment, only the source is removed.
func TransferCert(fromUserID, toUserID int64, certName string) error {
	tx, err := orm.NewOrm().Begin()
	if err != nil {
		return err
	}
	if _, err := tx.QueryTable(new(UserCertificate)).
		Filter("UserId", fromUserID).Filter("CertName", certName).Delete(); err != nil {
		_ = tx.Rollback()
		return err
	}
	if !tx.QueryTable(new(UserCertificate)).
		Filter("UserId", toUserID).Filter("CertName", certName).Exist() {
		if _, err := tx.Insert(&UserCertificate{UserId: toUserID, CertName: certName}); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// SeedAssignmentsFromLogin creates cert assignments for non-admin users where
// user.Login matches a cert CN. Skips existing assignments. Returns count created.
func SeedAssignmentsFromLogin(certNames []string) (int, error) {
	o := orm.NewOrm()
	var users []User
	if _, err := o.QueryTable(new(User)).Filter("IsAdmin", false).All(&users); err != nil {
		return 0, err
	}
	certSet := make(map[string]bool, len(certNames))
	for _, n := range certNames {
		certSet[n] = true
	}
	created := 0
	for _, u := range users {
		if !certSet[u.Login] {
			continue
		}
		if err := AssignCert(u.Id, u.Login); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}
