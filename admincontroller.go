package main

import (
	"fmt"
	"github.com/pki-io/core/config"
	"github.com/pki-io/core/document"
	"github.com/pki-io/core/entity"
)

const (
	AdminConfigFile string = "admin.conf"
)

type AdminParams struct {
	name          *string
	inviteId      *string
	inviteKey     *string
	confirmDelete *string
}

func NewAdminParams() *AdminParams {
	return new(AdminParams)
}

func (params *AdminParams) ValidateName(required bool) error {
	if required && *params.name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	return nil
}

func (params *AdminParams) ValidateInviteId(required bool) error  { return nil }
func (params *AdminParams) ValidateInviteKey(required bool) error { return nil }

type AdminController struct {
	env    *Environment
	config *config.AdminConfig
	admin  *entity.Entity
}

func NewAdminController(env *Environment) (*AdminController, error) {
	cont := new(AdminController)
	cont.env = env

	return cont, nil
}

func (cont *AdminController) LoadConfig() error {
	var err error
	if cont.config == nil {
		cont.config, err = config.NewAdmin()
		if err != nil {
			return err
		}
	}

	exists, err := cont.env.fs.home.Exists(AdminConfigFile)
	if err != nil {
		return err
	}

	if exists {
		adminConfig, err := cont.env.fs.home.Read(AdminConfigFile)
		if err != nil {
			return err
		}

		err = cont.config.Load(adminConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cont *AdminController) SaveConfig() error {
	cfgString, err := cont.config.Dump()
	if err != nil {
		return err
	}

	if err := cont.env.fs.home.Write(AdminConfigFile, cfgString); err != nil {
		return err
	}

	return nil
}

func (cont *AdminController) CreateAdmin(name string) error {

	var err error

	// TODO validate name
	cont.admin, err = entity.New(nil)
	if err != nil {
		return err
	}

	cont.admin.Data.Body.Id = NewID()
	cont.admin.Data.Body.Name = name
	err = cont.admin.GenerateKeys()
	if err != nil {
		return err
	}

	return nil
}

func (cont *AdminController) LoadAdmin() error {
	orgName := cont.env.controllers.org.config.Data.Name

	adminOrgConfig, err := cont.config.GetOrg(orgName)
	if err != nil {
		return err
	}

	adminId := adminOrgConfig.AdminId

	adminEntity, err := cont.env.fs.home.Read(adminId)
	if err != nil {
		return err
	}

	cont.admin, err = entity.New(adminEntity)
	if err != nil {
		return err
	}

	return nil
}

func (cont *AdminController) GetAdmin(id string) (*entity.Entity, error) {
	adminJson, err := cont.env.api.GetPublic(id, id)
	if err != nil {
		return nil, err
	}

	admin, err := entity.New(adminJson)
	if err != nil {
		return nil, err
	}

	return admin, nil
}

func (cont *AdminController) GetAdmins() ([]*entity.Entity, error) {
	index, err := cont.env.controllers.org.GetIndex()
	if err != nil {
		return nil, err
	}

	adminIds, err := index.GetAdmins()
	if err != nil {
		return nil, err
	}

	admins := make([]*entity.Entity, 0, 0)
	for _, id := range adminIds {
		admin, err := cont.GetAdmin(id)
		if err != nil {
			return nil, err
		}

		admins = append(admins, admin)
	}

	return admins, nil
}

func (cont *AdminController) SaveAdmin() error {
	id := cont.admin.Data.Body.Id

	// Save private admin to home
	if err := cont.env.fs.home.Write(id, cont.admin.Dump()); err != nil {
		return err
	}

	// Send a public admin
	if err := cont.env.api.SendPublic(id, id, cont.admin.DumpPublic()); err != nil {
		return err
	}

	return nil
}

func (cont *AdminController) SendOrgEntity() error {
	org := cont.env.controllers.org.org
	orgId := org.Data.Body.Id

	admins, err := cont.GetAdmins()
	if err != nil {
		return err
	}

	container, err := org.EncryptThenSignString(org.Dump(), admins)
	if err != nil {
		return err
	}

	if err := cont.env.api.SendPrivate(orgId, orgId, container.Dump()); err != nil {
		return err
	}

	return nil
}

func (cont *AdminController) SecureSendPublicToOrg(id, key string) error {

	cont.env.logger.Debug("Encrypting admin for org")
	container, err := cont.admin.EncryptThenAuthenticateString(cont.admin.DumpPublic(), id, key)
	if err != nil {
		return err
	}
	orgId := cont.env.controllers.org.config.Data.Id
	if err := cont.env.api.PushIncoming(orgId, "invite", container.Dump()); err != nil {
		return err
	}

	return nil
}

func (cont *AdminController) ProcessNextInvite() error {

	org := cont.env.controllers.org.org
	orgId := org.Data.Body.Id

	index, err := cont.env.controllers.org.GetIndex()
	if err != nil {
		return err
	}

	inviteJson, err := cont.env.api.PopIncoming(orgId, "invite")
	if err != nil {
		return err
	}

	container, err := document.NewContainer(inviteJson)
	if err != nil {
		cont.env.api.PushIncoming(orgId, "invite", inviteJson)
		return err
	}

	inviteId := container.Data.Options.SignatureInputs["key-id"]
	cont.env.logger.Debugf("Reading invite key: %s", inviteId)
	inviteKey, err := index.GetInviteKey(inviteId)
	if err != nil {
		cont.env.api.PushIncoming(orgId, "invite", inviteJson)
		return err
	}

	cont.env.logger.Debug("Verifying and decrypting admin invite")
	adminJson, err := org.VerifyAuthenticationThenDecrypt(container, inviteKey.Key)
	if err != nil {
		cont.env.api.PushIncoming(orgId, "invite", inviteJson)
		return err
	}

	admin, err := entity.New(adminJson)
	if err != nil {
		cont.env.api.PushIncoming(orgId, "invite", inviteJson)
		return err
	}

	if err := index.AddAdmin(admin.Data.Body.Name, admin.Data.Body.Id); err != nil {
		return err
	}

	if err := cont.env.controllers.org.SaveIndex(index); err != nil {
		return err
	}

	if err := cont.SendOrgEntity(); err != nil {
		return err
	}

	orgContainer, err := cont.admin.EncryptThenAuthenticateString(org.DumpPublic(), inviteId, inviteKey.Key)
	if err != nil {
		return err
	}

	if err := cont.env.api.PushIncoming(admin.Data.Body.Id, "invite", orgContainer.Dump()); err != nil {
		return err
	}

	// Delete invite ID

	return nil
}

func (cont *AdminController) ProcessInvites() error {
	cont.env.logger.Debug("Processing invites")

	orgId := cont.env.controllers.org.org.Data.Body.Id
	for {
		size, err := cont.env.api.IncomingSize(orgId, "invite")
		if err != nil {
			return err
		}

		cont.env.logger.Debugf("Found %d invites to process", size)

		if size > 0 {
			if err := cont.ProcessNextInvite(); err != nil {
				return err
			}
		} else {
			break
		}
	}

	return nil
}

func (cont *AdminController) ShowEnv(params *AdminParams) error {
	index, err := cont.env.controllers.org.GetIndex()
	if err != nil {
		return err
	}

	adminId, err := index.GetAdmin(*params.name)
	if err != nil {
		return err
	}

	admin, err := cont.GetAdmin(adminId)
	if err != nil {
		return err
	}

	cont.env.logger.Info("Showing admin:")
	cont.env.logger.Flush()

	fmt.Printf("Name: %s\n", admin.Data.Body.Name)
	fmt.Printf("ID: %s\n", admin.Data.Body.Id)
	fmt.Printf("Key type: %s\n", admin.Data.Body.KeyType)
	fmt.Printf("Public encryption key:\n%s\n", admin.Data.Body.PublicEncryptionKey)
	fmt.Printf("Public signing key:\n%s\n", admin.Data.Body.PublicSigningKey)

	return nil
}

func (cont *AdminController) InviteEnv(params *AdminParams) error {

	cont.env.logger.Debug("Creating new admin key")
	id := NewID()
	key := NewID()

	cont.env.logger.Debug("Saving key to index")
	index, err := cont.env.controllers.org.GetIndex()
	if err != nil {
		return err
	}

	index.AddInviteKey(id, key, *params.name)

	if err := cont.env.controllers.org.SaveIndex(index); err != nil {
		return err
	}

	cont.env.logger.Info("Creating invite")
	cont.env.logger.Flush()

	fmt.Printf("Invite ID: %s\n", id)
	fmt.Printf("Invite key: %s\n", key)

	return nil
}

func (cont *AdminController) RunEnv(params *AdminParams) error {

	if err := cont.ProcessInvites(); err != nil {
		return err
	}

	return nil
}

func (cont *AdminController) List(params *AdminParams) error {
	cont.env.logger.Debug("Loading admin environment")

	if err := cont.env.LoadAdminEnv(); err != nil {
		return err
	}

	index, err := cont.env.controllers.org.GetIndex()
	if err != nil {
		return err
	}

	admins, err := index.GetAdmins()
	if err != nil {
		return err
	}

	cont.env.logger.Info("Listing admins:")
	cont.env.logger.Flush()

	for name, id := range admins {
		fmt.Printf("* %s %s\n", name, id)
	}

	return nil
}

func (cont *AdminController) Show(params *AdminParams) error {
	cont.env.logger.Debug("Validating parameters")

	if err := params.ValidateName(true); err != nil {
		return err
	}

	cont.env.logger.Debug("Loading admin environment")

	if err := cont.env.LoadAdminEnv(); err != nil {
		return err
	}

	return cont.env.controllers.admin.ShowEnv(params)
}

func (cont *AdminController) Invite(params *AdminParams) error {

	cont.env.logger.Debug("Validating parameters")

	if err := params.ValidateName(true); err != nil {
		return err
	}

	cont.env.logger.Debug("Loading admin environment")

	if err := cont.env.LoadAdminEnv(); err != nil {
		return err
	}

	return cont.env.controllers.admin.InviteEnv(params)
}

func (cont *AdminController) New(params *AdminParams) error {

	var err error

	cont.env.logger.Debug("Validating parameters")

	if err := params.ValidateName(true); err != nil {
		return err
	}

	cont.env.logger.Debug("Loading local filesystem")
	if err := cont.env.LoadLocalFs(); err != nil {
		return err
	}

	cont.env.logger.Debug("Loading home filesystem")
	if err := cont.env.LoadHomeFs(); err != nil {
		return err
	}

	cont.env.logger.Debug("Loading API")
	if err := cont.env.LoadAPI(); err != nil {
		return err
	}

	cont.env.logger.Debug("Initializing org controller")
	if cont.env.controllers.org == nil {
		if cont.env.controllers.org, err = NewOrgController(cont.env); err != nil {
			return err
		}
	}

	cont.env.logger.Debug("Loading org config")
	if err := cont.env.controllers.org.LoadConfig(); err != nil {
		return err
	}

	cont.env.logger.Debug("Creating admin entity")
	cont.admin, err = entity.New(nil)
	if err != nil {
		return nil
	}

	cont.admin.Data.Body.Id = NewID()
	cont.admin.Data.Body.Name = *params.name

	cont.env.logger.Debug("Generating admin keys")
	if err := cont.admin.GenerateKeys(); err != nil {
		return err
	}

	if err := cont.SaveAdmin(); err != nil {
		return nil
	}

	if err := cont.LoadConfig(); err != nil {
		return err
	}

	orgId := cont.env.controllers.org.config.Data.Id
	orgName := cont.env.controllers.org.config.Data.Name

	if err := cont.config.AddOrg(orgName, orgId, cont.admin.Data.Body.Id); err != nil {
		return err
	}

	if err := cont.SaveConfig(); err != nil {
		return err
	}

	if err := cont.SecureSendPublicToOrg(*params.inviteId, *params.inviteKey); err != nil {
		return err
	}

	return nil
}

func (cont *AdminController) Run(params *AdminParams) error {

	if err := cont.env.LoadAdminEnv(); err != nil {
		return err
	}

	return cont.env.controllers.admin.RunEnv(params)
}

func (cont *AdminController) Complete(params *AdminParams) error {

	var err error

	cont.env.logger.Debug("Validating parameters")

	cont.env.logger.Debug("Loading local filesystem")
	if err := cont.env.LoadLocalFs(); err != nil {
		return err
	}

	cont.env.logger.Debug("Loading home filesystem")
	if err := cont.env.LoadHomeFs(); err != nil {
		return err
	}

	cont.env.logger.Debug("Loading API")
	if err := cont.env.LoadAPI(); err != nil {
		return err
	}

	cont.env.logger.Debug("Initializing org controller")
	if cont.env.controllers.org == nil {
		if cont.env.controllers.org, err = NewOrgController(cont.env); err != nil {
			return err
		}
	}

	cont.env.logger.Debug("Loading org config")
	if err := cont.env.controllers.org.LoadConfig(); err != nil {
		return err
	}

	if err := cont.LoadConfig(); err != nil {
		return err
	}

	if err := cont.LoadAdmin(); err != nil {
		return err
	}

	orgContainerJson, err := cont.env.api.PopIncoming(cont.admin.Data.Body.Id, "invite")
	if err != nil {
		return err
	}

	orgContainer, err := document.NewContainer(orgContainerJson)
	if err != nil {
		return err
	}

	orgJson, err := cont.admin.VerifyAuthenticationThenDecrypt(orgContainer, *params.inviteKey)
	if err != nil {
		return err
	}

	org, err := entity.New(orgJson)
	if err != nil {
		return err
	}

	cont.env.logger.Debug("Saving public org to home")
	if err := cont.env.fs.home.Write(org.Data.Body.Id, org.DumpPublic()); err != nil {
		return err
	}

	return nil
}

func (cont *AdminController) Delete(params *AdminParams) error {
	cont.env.logger.Debug("Loading admin environment")

	if err := cont.env.LoadAdminEnv(); err != nil {
		return err
	}

	index, err := cont.env.controllers.org.GetIndex()
	if err != nil {
		return err
	}

	if err := index.RemoveAdmin(*params.name); err != nil {
		return err
	}

	if err := cont.env.controllers.org.SaveIndex(index); err != nil {
		return err
	}

	if err := cont.SendOrgEntity(); err != nil {
		return err
	}

	return nil
}