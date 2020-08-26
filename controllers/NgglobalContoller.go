package controllers

import (
	"fmt"
	ngJson "github.com/ClessLi/bifrost/pkg/json/nginx"
	"github.com/ClessLi/bifrost/pkg/resolv/nginx"
	"github.com/astaxie/beego/logs"
	"strconv"
	"strings"

	"Heimdallr/enums"
	"Heimdallr/models"
	"Heimdallr/utils"

	"github.com/astaxie/beego/orm"
)

type NgglobalController struct {
	BaseController
}

type Returngobal struct {
	Key   string
	Value string
	Ison  bool // 是否启用，no为注释#
}

func (c *NgglobalController) Prepare() {
	//先执行
	c.BaseController.Prepare()
	//如果一个Controller的多数Action都需要权限控制，则将验证放到Prepare
	c.checkAuthor("DataGrid")
	//如果一个Controller的所有Action都需要登录验证，则将验证放到Prepare
	//权限控制里会进行登录验证，因此这里不用再作登录验证
	//c.checkLogin()

}
func (c *NgglobalController) Index() {

	//将页面左边菜单的某项激活
	c.Data["activeSidebarUrl"] = c.URLFor(c.controllerName + "." + c.actionName)
	//页面模板设置
	c.setTpl()
	c.LayoutSections = make(map[string]string)
	c.LayoutSections["headcssjs"] = "ngglobal/index_headcssjs.html"
	c.LayoutSections["footerjs"] = "ngglobal/index_footerjs.html"
	//页面里按钮权限控制
	c.Data["canEdit"] = c.checkActionAuthor("NgglobalController", "Edit")
	c.Data["canDelete"] = c.checkActionAuthor("NgglobalController", "Delete")
	c.Data["canSave"] = c.checkActionAuthor("NgglobalController", "Save")
}

func (c *NgglobalController) DataGrid() {

	// 定义是否启用的状态
	var returngobal []Returngobal
	var gobal Returngobal
	// 获取nginx conf结构体数据
	var Ngjson *string
	m, b := c.EnvCookie()
	if b {
		Ngjson = models.GetDetail(m, "json")
	}

	conf, err := ngJson.Unmarshal([]byte(*Ngjson))

	if err != nil {
		logs.Error(err)
	}

	res := conf.(*nginx.Config).Params()

	for _, data := range res {
		// 获取到当前一条数据
		switch data.(type) {
		case *nginx.Key:
			gobal.Key = data.(*nginx.Key).Name
			gobal.Value = data.(*nginx.Key).Value
			gobal.Ison = true
			returngobal = append(returngobal, gobal)
			//case *nginx.Comment:
			//TODO: 注释暂时不展示
		}

	}
	result := make(map[string]interface{})
	result["total"] = len(returngobal)
	result["rows"] = returngobal
	c.Data["json"] = result
	c.ServeJSON()
}

// Edit 添加 编辑 页面
func (c *NgglobalController) Edit() {

	Id, _ := c.GetInt(":id", 0)
	m := &models.BackendUser{}
	var err error
	if Id > 0 {
		m, err = models.BackendUserOne(Id)
		if err != nil {
			c.pageError("数据无效，请刷新后重试")
		}
		o := orm.NewOrm()
		o.LoadRelated(m, "RoleBackendUserRel")
	} else {
		//添加用户时默认状态为启用
		m.Status = enums.Enabled
	}
	c.Data["m"] = m
	//获取关联的roleId列表
	var roleIds []string
	for _, item := range m.RoleBackendUserRel {
		roleIds = append(roleIds, strconv.Itoa(item.Role.Id))
	}
	c.Data["roles"] = strings.Join(roleIds, ",")
	c.setTpl("ngglobal/edit.html", "shared/layout_pullbox.html")
	c.LayoutSections = make(map[string]string)
	c.LayoutSections["footerjs"] = "ngglobal/edit_footerjs.html"
}
func (c *NgglobalController) Save() {
	m := models.BackendUser{}
	o := orm.NewOrm()
	var err error
	//获取form里的值
	if err = c.ParseForm(&m); err != nil {
		c.jsonResult(enums.JRCodeFailed, "获取数据失败", m.Id)
	}
	//删除已关联的历史数据
	if _, err := o.QueryTable(models.RoleBackendUserRelTBName()).Filter("backenduser__id", m.Id).Delete(); err != nil {
		c.jsonResult(enums.JRCodeFailed, "删除历史关系失败", "")
	}
	if m.Id == 0 {
		//对密码进行加密
		m.UserPwd = utils.String2md5(m.UserPwd)
		if _, err := o.Insert(&m); err != nil {
			c.jsonResult(enums.JRCodeFailed, "添加失败", m.Id)
		}
	} else {
		if oM, err := models.BackendUserOne(m.Id); err != nil {
			c.jsonResult(enums.JRCodeFailed, "数据无效，请刷新后重试", m.Id)
		} else {
			m.UserPwd = strings.TrimSpace(m.UserPwd)
			if len(m.UserPwd) == 0 {
				//如果密码为空则不修改
				m.UserPwd = oM.UserPwd
			} else {
				m.UserPwd = utils.String2md5(m.UserPwd)
			}
			//本页面不修改头像和密码，直接将值附给新m
			m.Avatar = oM.Avatar
		}
		if _, err := o.Update(&m); err != nil {
			c.jsonResult(enums.JRCodeFailed, "编辑失败", m.Id)
		}
	}
	//添加关系
	var relations []models.RoleBackendUserRel
	for _, roleId := range m.RoleIds {
		r := models.Role{Id: roleId}
		relation := models.RoleBackendUserRel{BackendUser: &m, Role: &r}
		relations = append(relations, relation)
	}
	if len(relations) > 0 {
		//批量添加
		if _, err := o.InsertMulti(len(relations), relations); err == nil {
			c.jsonResult(enums.JRCodeSucc, "保存成功", m.Id)
		} else {
			c.jsonResult(enums.JRCodeFailed, "保存失败", m.Id)
		}
	} else {
		c.jsonResult(enums.JRCodeSucc, "保存成功", m.Id)
	}
}
func (c *NgglobalController) Delete() {
	strs := c.GetString("ids")
	ids := make([]int, 0, len(strs))
	for _, str := range strings.Split(strs, ",") {
		if id, err := strconv.Atoi(str); err == nil {
			ids = append(ids, id)
		}
	}
	query := orm.NewOrm().QueryTable(models.BackendUserTBName())
	if num, err := query.Filter("id__in", ids).Delete(); err == nil {
		c.jsonResult(enums.JRCodeSucc, fmt.Sprintf("成功删除 %d 项", num), 0)
	} else {
		c.jsonResult(enums.JRCodeFailed, "删除失败", 0)
	}
}