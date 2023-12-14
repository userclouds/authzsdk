package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/gofrs/uuid"
	"github.com/joho/godotenv"

	"userclouds.com/idp"
	"userclouds.com/idp/policy"
	"userclouds.com/idp/userstore"
	"userclouds.com/infra/jsonclient"
)

// This sample shows you how to create new columns in the user store and create access policies governing access
// to the data inside those columns. It also shows you how to create, delete and execute accessors and mutators.
// To learn more about these concepts, see docs.userclouds.com.

func setup(ctx context.Context) (*idp.Client, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {

	err := godotenv.Load()
	if err != nil {
		log.Printf("error loading .env file: %v\n(did you forget to copy `.env.example` to `.env` and substitute values?)", err)
	}

	tenantURL := os.Getenv("TENANT_URL")
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")

	if tenantURL == "" || clientID == "" || clientSecret == "" {
		log.Fatal("missing one or more required environment variables: TENANT_URL, CLIENT_ID, CLIENT_SECRET")
	}

	// reusable token source
	ts, err := jsonclient.ClientCredentialsForURL(tenantURL, clientID, clientSecret, nil)
	if err != nil {
		panic(err)
	}
	// we'll need IDP and tokenizer clients
	idpc, err := idp.NewClient(tenantURL, idp.JSONClient(ts, jsonclient.StopLogging()))
	if err != nil {
		panic(err)
	}

	tc := idpc.TokenizerClient

	// create phone number and address columns
	if _, err := idpc.CreateColumn(ctx, userstore.Column{
		Name:      "phone_number",
		Type:      userstore.DataTypeString,
		IndexType: userstore.ColumnIndexTypeIndexed,
	}, idp.IfNotExists()); err != nil {
		panic(err)
	}
	if _, err := idpc.CreateColumn(ctx, userstore.Column{
		Name:      "home_addresses",
		Type:      userstore.DataTypeAddress,
		IsArray:   true,
		IndexType: userstore.ColumnIndexTypeNone,
	}, idp.IfNotExists()); err != nil {
		panic(err)
	}

	var aptID uuid.UUID
	apt := &policy.AccessPolicyTemplate{
		Name: "CheckAPIKey",
		Function: `function policy(context, params) {
			return context.client.api_key == params.api_key;
		}`,
	}
	if apt, err = tc.CreateAccessPolicyTemplate(ctx, *apt, idp.IfNotExists()); err != nil {
		panic(err)
	} else {
		aptID = apt.ID
	}

	var apID uuid.UUID
	ap := &policy.AccessPolicy{
		Name: "CheckAPIKeySecurityTeam",
		Components: []policy.AccessPolicyComponent{{Template: &userstore.ResourceID{ID: aptID},
			TemplateParameters: `{"api_key": "api_key_for_security_team"}`}},
		PolicyType: policy.PolicyTypeCompositeIntersection,
	}
	if ap, err = tc.CreateAccessPolicy(ctx, *ap, idp.IfNotExists()); err != nil {
		panic(err)
	} else {
		apID = ap.ID
	}

	// Purposes are used to specify the purpose for which a client is requesting access to PII. They are assigned
	// in mutators based on an action of user consent, and they are checked in mutators and accessors to ensure
	// that the client has access to the data. More on accessors and mutators below. We need to create a purpose
	// for the security team to access PII for security reasons, as well as an "operational" purpose for the
	// general operations of the site.
	securityPurpose, err := idpc.CreatePurpose(ctx, userstore.Purpose{
		Name:        "security",
		Description: "This purpose is used to grant access to PII for security reasons.",
	}, idp.IfNotExists())
	if err != nil {
		panic(err)
	}
	operationalPurpose, err := idpc.CreatePurpose(ctx, userstore.Purpose{
		Name:        "operational",
		Description: "This purpose is used to grant access to PII for general site operations.",
	}, idp.IfNotExists())
	if err != nil {
		panic(err)
	}

	// Accessors are configurable APIs that allow a client to retrieve data from the user store. Accessors are
	// intended to be use-case specific. They enforce data usage policies and minimize outbound data from the
	// store for their given use case.

	// Selectors are used to filter the set of users that are returned by an accessor. They are eseentially SQL
	// WHERE clauses and are configured per-accessor / per-mutator referencing column IDs of the userstore.

	accessor := &userstore.Accessor{
		ID:                 uuid.Nil,
		Name:               "AccessorForSecurity",
		DataLifeCycleState: userstore.DataLifeCycleStateLive,
		Columns: []userstore.ColumnOutputConfig{
			{Column: userstore.ResourceID{Name: "id"}, Transformer: userstore.ResourceID{ID: policy.TransformerPassthrough.ID}},
			{Column: userstore.ResourceID{Name: "updated"}, Transformer: userstore.ResourceID{ID: policy.TransformerPassthrough.ID}},
			{Column: userstore.ResourceID{Name: "phone_number"}, Transformer: userstore.ResourceID{ID: policy.TransformerPassthrough.ID}},
			{Column: userstore.ResourceID{Name: "home_addresses"}, Transformer: userstore.ResourceID{ID: policy.TransformerPassthrough.ID}},
		},
		AccessPolicy: userstore.ResourceID{ID: apID},
		SelectorConfig: userstore.UserSelectorConfig{
			WhereClause: "{home_addresses}->>'street_address_line_1' LIKE ?",
		},
		Purposes: []userstore.ResourceID{{ID: securityPurpose.ID}, {ID: operationalPurpose.ID}},
	}
	if accessor, err = idpc.CreateAccessor(ctx, *accessor, idp.IfNotExists()); err != nil {
		panic(err)
	}

	// Transformers are used to transform data in the user store. There are 4 different types of transformers:
	// - Passthrough - doesn't modify the data
	// - Transform - transforms the data into a value not resolvable to original value like extracting area code from phone number
	// - TokenizeByValue - transforms the data into a token that is resolvable to the value passed in
	// - TokenizeByReference - transforms the data into a token that is resolvable to the value stored in input column at the time of resolution

	// Code for transformer that will tokenize phone number into a token which a random sequence of digits in a format of a phone number
	transformerBody := `function id(len) {
		var s = "0123456789";
		return Array(len).join().split(',').map(function() {
			return s.charAt(Math.floor(Math.random() * s.length));
		}).join('');
	}
	function validate(str) {
		return (str.length === 10);
	}
	function transform(data, params) {
	  // Strip non numeric characters if present
	  orig_data = data;
	  data = data.replace(/\D/g, '');
	  if (data.length === 11 ) {
		data = data.substr(1, 11);
	  }
	  if (!validate(data)) {
			throw new Error('Invalid US Phone Number Provided');
	  }
	  return '1' + id(10);
	}`

	// Define a transformer  that will tokenize phone number into a fixed token (i.e. phone number A always maps to token A)
	phoneNumberTokenTransformer := &policy.Transformer{
		Name:               "TokenizePhoneByValReuse",
		Function:           transformerBody,
		Parameters:         ``,
		InputType:          userstore.DataTypeString,
		OutputType:         userstore.DataTypeString,
		ReuseExistingToken: true, // Fixed mapping
		TransformType:      policy.TransformTypeTokenizeByValue,
	}

	phoneNumberTokenTransformer, err = tc.CreateTransformer(ctx, *phoneNumberTokenTransformer, idp.IfNotExists())
	if err != nil {
		panic(err)
	}

	// Define a transformer  that will transfomr phone number into a country code (i.e. phone number A -> country code)
	phoneNumberTransformer := &policy.Transformer{
		Name:        "TransformPhoneNumberToCountryNamev2",
		Description: "This transformer gets the country name for the phone number",
		InputType:   userstore.DataTypeString,
		OutputType:  userstore.DataTypeString,
		Function: `function transform(data, params) {
			try {
				return getCountryNameForPhoneNumber(data);
			} catch (e) {
				return "";
			}
		}`,
		Parameters:    ``,
		TransformType: policy.TransformTypeTransform,
	}

	phoneNumberTransformer, err = tc.CreateTransformer(ctx, *phoneNumberTransformer, idp.IfNotExists())
	if err != nil {
		panic(err)
	}

	addressTransformer := &policy.Transformer{
		Name:        "AddressCountryOnlyV2",
		Description: "This transformer returns just the zip code of the address.",
		InputType:   userstore.DataTypeAddress,
		OutputType:  userstore.DataTypeString,
		Function: `function transform(data, params) {
			return data.country;
		}`,
		Parameters:    ``,
		TransformType: policy.TransformTypeTransform,
	}

	addressTransformer, err = tc.CreateTransformer(ctx, *addressTransformer, idp.IfNotExists())
	if err != nil {
		panic(err)
	}

	tokenResolvePolicy := &policy.AccessPolicy{
		Name: "CheckAPIKeyDataScienceTeam",
		Components: []policy.AccessPolicyComponent{{Template: &userstore.ResourceID{ID: aptID},
			TemplateParameters: `{"api_key": "api_key_for_data_science"}`}},
		PolicyType: policy.PolicyTypeCompositeIntersection,
	}

	if tokenResolvePolicy, err = tc.CreateAccessPolicy(ctx, *tokenResolvePolicy, idp.IfNotExists()); err != nil {
		panic(err)
	}

	dsAccessor := &userstore.Accessor{
		ID:                 uuid.Nil,
		Name:               "AccessorForDataScienceV3",
		DataLifeCycleState: userstore.DataLifeCycleStateLive,
		SelectorConfig: userstore.UserSelectorConfig{
			WhereClause: "{phone_number} != ?",
		},
		Columns: []userstore.ColumnOutputConfig{
			{Column: userstore.ResourceID{Name: "phone_number"}, Transformer: userstore.ResourceID{ID: phoneNumberTransformer.ID}},
			{Column: userstore.ResourceID{Name: "home_addresses"}, Transformer: userstore.ResourceID{ID: addressTransformer.ID}},
		},
		AccessPolicy:      userstore.ResourceID{ID: policy.AccessPolicyAllowAll.ID},
		TokenAccessPolicy: userstore.ResourceID{ID: tokenResolvePolicy.ID},
		Purposes:          []userstore.ResourceID{{ID: operationalPurpose.ID}},
	}
	if dsAccessor, err = idpc.CreateAccessor(ctx, *dsAccessor, idp.IfNotExists()); err != nil {
		panic(err)
	}

	// Define accessor that can be used to log user data safely while not exposing user information and using a fixed phone number token to identify the user in logs
	securityLoggingAccessor := &userstore.Accessor{
		ID: uuid.Must(uuid.NewV4()),
		Columns: []userstore.ColumnOutputConfig{
			{Column: userstore.ResourceID{Name: "email"}, Transformer: userstore.ResourceID{ID: uuid.Must(uuid.FromString("0cedf7a4-86ab-450a-9426-478ad0a60faa"))}},
			{Column: userstore.ResourceID{Name: "phone_number"}, Transformer: userstore.ResourceID{ID: phoneNumberTokenTransformer.ID}},
		},
		Name:               "AccessorForSecurityLogging",
		DataLifeCycleState: userstore.DataLifeCycleStateLive,
		AccessPolicy:       userstore.ResourceID{ID: policy.AccessPolicyAllowAll.ID},
		TokenAccessPolicy:  userstore.ResourceID{ID: apID},
		SelectorConfig:     userstore.UserSelectorConfig{WhereClause: "{id} = ANY(?)"},
		Purposes:           []userstore.ResourceID{{ID: operationalPurpose.ID}},
	}

	if securityLoggingAccessor, err = idpc.CreateAccessor(ctx, *securityLoggingAccessor, idp.IfNotExists()); err != nil || securityLoggingAccessor == nil {
		panic(err)
	}
	// Mutators are configurable APIs that allow a client to write data to the User Store. Mutators (setters)
	// can be thought of as the complement to accessors (getters).

	mutator := &userstore.Mutator{
		ID: uuid.Nil,
		Columns: []userstore.ColumnInputConfig{
			{Column: userstore.ResourceID{Name: "phone_number"}, Normalizer: userstore.ResourceID{ID: policy.TransformerPassthrough.ID}},
			{Column: userstore.ResourceID{Name: "home_addresses"}, Normalizer: userstore.ResourceID{ID: policy.TransformerPassthrough.ID}},
		},
		Name:         "MutatorForSecurityTeam",
		AccessPolicy: userstore.ResourceID{ID: apID},
		SelectorConfig: userstore.UserSelectorConfig{
			WhereClause: "{id} = ?",
		},
	}
	if mutator, err = idpc.CreateMutator(ctx, *mutator, idp.IfNotExists()); err != nil {
		panic(err)
	}

	return idpc, accessor.ID, mutator.ID, dsAccessor.ID, securityLoggingAccessor.ID
}

func example(ctx context.Context, idpc *idp.Client, accessorID, mutatorID, dsAccessorID uuid.UUID, logAccessor uuid.UUID) {

	uid, err := idpc.CreateUser(ctx, userstore.Record{"email": "test@test.org"})
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := idpc.DeleteUser(ctx, uid); err != nil {
			panic(err)
		}
	}()

	// add the phone number or address using a mutator, but only store with "operational" purpose
	if _, err := idpc.ExecuteMutator(ctx,
		mutatorID,
		policy.ClientContext{"api_key": "api_key_for_security_team"},
		[]interface{}{uid.String()},
		map[string]idp.ValueAndPurposes{
			"phone_number": {
				Value:            "14155555555",
				PurposeAdditions: []userstore.ResourceID{{Name: "operational"}},
			},
			"home_addresses": {
				Value: `[{"country":"usa", "street_address_line_1":"742 Evergreen Terrace", "locality":"Springfield"},
						 {"country":"usa", "street_address_line_1":"123 Main St", "locality":"Pleasantville"}]`,
				PurposeAdditions: []userstore.ResourceID{{Name: "operational"}},
			},
		}); err != nil {

		panic(err)
	}

	// use security accessor to get the user details
	resolved, err := idpc.ExecuteAccessor(ctx,
		accessorID,
		policy.ClientContext{"api_key": "api_key_for_security_team"},
		[]interface{}{"%Evergreen%"})
	if err != nil {
		panic(err)
	}

	fmt.Printf("user details for security team are (after first mutator):\n%v\n\n", resolved)

	// add the phone number or address using a mutator, adding the "security" purpose
	if _, err := idpc.ExecuteMutator(ctx,
		mutatorID,
		policy.ClientContext{"api_key": "api_key_for_security_team"},
		[]interface{}{uid.String()},
		map[string]idp.ValueAndPurposes{
			"phone_number": {
				Value:            "14155555555",
				PurposeAdditions: []userstore.ResourceID{{Name: "security"}},
			},
			"home_addresses": {
				Value: `[{"country":"usa", "street_address_line_1":"742 Evergreen Terrace", "locality":"Springfield"},
						 {"country":"usa", "street_address_line_1":"123 Main St", "locality":"Pleasantville"}]`,
				PurposeAdditions: []userstore.ResourceID{{Name: "security"}},
			},
		}); err != nil {

		panic(err)
	}

	// use security accessor to get the user details a second time
	resolved, err = idpc.ExecuteAccessor(ctx,
		accessorID,
		policy.ClientContext{"api_key": "api_key_for_security_team"},
		[]interface{}{"%Evergreen%"})
	if err != nil {
		panic(err)
	}

	fmt.Printf("user details for security are (after second mutator):\n%v\n\n", resolved)

	// use data science accessor to get the user details
	resolved, err = idpc.ExecuteAccessor(ctx,
		dsAccessorID,
		policy.ClientContext{"api_key": "api_key_for_data_science_team"},
		[]interface{}{"usa"})
	if err != nil {
		panic(err)
	}

	fmt.Printf("user details for data science are:\n%v\n\n", resolved)

	// use security logging accessor to get the user details for logs emulating two different calls sites
	resolved, err = idpc.ExecuteAccessor(ctx,
		logAccessor,
		policy.ClientContext{"api_key": "api_key_for_data_science_team"},
		[]interface{}{[]uuid.UUID{uid}})
	if err != nil {
		panic(err)
	}

	fmt.Printf("user details for used for logging at call site 1 are:\n%v\n\n", resolved)
	resolved, err = idpc.ExecuteAccessor(ctx,
		logAccessor,
		policy.ClientContext{"api_key": "api_key_for_data_science_team"},
		[]interface{}{[]uuid.UUID{uid}})
	if err != nil {
		panic(err)
	}

	fmt.Printf("user details for used for logging at call site 2:\n%v\n\n", resolved)

	// Resolve the tokenized phone numbers from logs to the original phone numbers for security team
	for _, value := range resolved.Data {
		var m map[string]string
		if err = json.Unmarshal([]byte(value), &m); err != nil {
			panic(err)
		}
		phoneNumberToken := m["phone_number"]

		resolvedPhoneNumber, err := idpc.ResolveToken(ctx,
			phoneNumberToken,
			policy.ClientContext{"api_key": "api_key_for_security_team"},
			nil)
		if err != nil {
			panic(err)
		}

		fmt.Printf("resolving phone number token %s to phone number %s\n", phoneNumberToken, resolvedPhoneNumber)
	}
}

func cleanup(ctx context.Context, idpc *idp.Client, accessorID, mutatorID, dsAccessorID, logAccessor uuid.UUID) {
	if err := idpc.DeleteAccessor(ctx, accessorID); err != nil {
		panic(err)
	}
	if err := idpc.DeleteAccessor(ctx, dsAccessorID); err != nil {
		panic(err)
	}
	if err := idpc.DeleteAccessor(ctx, logAccessor); err != nil {
		panic(err)
	}
	if err := idpc.DeleteMutator(ctx, mutatorID); err != nil {
		panic(err)
	}
}

func main() {
	ctx := context.Background()

	idpc, accessorID, mutatorID, dsAccessorID, logAccessor := setup(ctx)

	example(ctx, idpc, accessorID, mutatorID, dsAccessorID, logAccessor)

	cleanup(ctx, idpc, accessorID, mutatorID, dsAccessorID, logAccessor)

}
