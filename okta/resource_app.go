package okta

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceApp() *schema.Resource {
	return &schema.Resource{
		Create: resourceAppCreate,
		Read:   resourceAppRead,
		Update: resourceAppUpdate,
		Delete: resourceAppDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"label": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"sign_on_mode": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "SAML_2_0",
			},
			"aws_environment_type": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"group_filter": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"login_url": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"join_all_roles": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"identity_provider_arn": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"session_duration": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  3600,
			},
			"role_value_pattern": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"saml_metadata_document": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"aws_okta_iam_user_id": &schema.Schema{
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},
			"aws_okta_iam_user_secret": &schema.Schema{
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},
		},
	}
}

func resourceAppCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(Config)
	client := config.Okta
	web := config.Web
	awsKey := d.Get("aws_okta_iam_user_id").(string)
	awsSecret := d.Get("aws_okta_iam_user_secret").(string)

	name := d.Get("label").(string)
	identityArn := d.Get("identity_provider_arn").(string)

	createdApplication, err := client.CreateAwsApplication(name, identityArn)
	if err != nil {
		return err
	}

	samlMetadataDocument, err := client.GetSAMLMetadata(createdApplication.ID, createdApplication.Credentials.Signing.KeyID)
	if err != nil {
		return err
	}

	if samlMetadataDocument == "" {
		return fmt.Errorf("Request for SAML returned nothing from app %s", createdApplication.ID)
	}

	provisionErr := web.SetAWSProvisioning(createdApplication.ID, awsKey, awsSecret)
	if provisionErr != nil {
		return provisionErr
	}

	fmt.Printf("%+v\n", createdApplication)
	d.SetId(createdApplication.ID)
	d.Set("saml_metadata_document", samlMetadataDocument)

	return nil
}

func resourceAppRead(d *schema.ResourceData, m interface{}) error {
	config := m.(Config)
	client := config.Okta
	appID := d.Id()

	readApplication, err := client.GetApplication(appID)
	if err != nil {
		return err
	}

	if readApplication == nil {
		log.Printf("[WARN] Okta Application %s (%q) not found, removing from state", d.Get("label").(string), d.Id())
		d.SetId("")
		return nil
	}

	samlMetadataDocument, err := client.GetSAMLMetadata(appID, readApplication.Credentials.Signing.KeyID)
	if err != nil {
		return err
	}

	d.Set("name", readApplication.Name)
	d.Set("label", readApplication.Label)
	d.Set("sign_on_mode", readApplication.SignOnMode)
	d.Set("aws_environment_type", readApplication.Settings.App.AwsEnvironmentType)
	d.Set("group_filter", readApplication.Settings.App.GroupFilter)
	d.Set("login_url", readApplication.Settings.App.LoginURL)
	d.Set("join_all_roles", readApplication.Settings.App.JoinAllRoles)
	d.Set("identity_provider_arn", readApplication.Settings.App.IdentityProviderArn)
	d.Set("session_duration", readApplication.Settings.App.SessionDuration)
	d.Set("role_value_pattern", readApplication.Settings.App.RoleValuePattern)
	d.Set("saml_metadata_document", samlMetadataDocument)

	fmt.Printf("%+v\n", readApplication)
	return nil
}

func resourceAppUpdate(d *schema.ResourceData, m interface{}) error {
	config := m.(Config)
	client := config.Okta
	web := config.Web

	awsKey := d.Get("aws_okta_iam_user_id").(string)
	awsSecret := d.Get("aws_okta_iam_user_secret").(string)

	name := d.Get("label").(string)
	identityArn := d.Get("identity_provider_arn").(string)

	updatedApplication, err := client.UpdateAwsApplication(d.Id(), name, identityArn)
	if err != nil {
		return err
	}

	err = web.RevokeAWSProvisioning(updatedApplication.ID)
	if err != nil {
		return err
	}

	time.Sleep(15 * time.Second)

	err = web.SetAWSProvisioning(updatedApplication.ID, awsKey, awsSecret)
	if err != nil {
		return err
	}

	log.Printf("[WARN] Credentials for okta updated with [%s]", awsKey)
	fmt.Printf("%+v\n", updatedApplication)
	d.SetId(updatedApplication.ID)

	return nil
}

func resourceAppDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(Config)
	client := config.Okta
	appID := d.Id()

	err := client.DeleteApplication(appID)

	if err != nil {
		return err
	}

	return nil
}
