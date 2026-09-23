package auth

import (
	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
)

var roleByName = map[string]krillv1.Role{
	"labeler":   krillv1.Role_ROLE_LABELER,
	"developer": krillv1.Role_ROLE_DEVELOPER,
	"admin":     krillv1.Role_ROLE_ADMIN,
}

func ParseRole(name string) krillv1.Role {
	return roleByName[name]
}

func RoleName(r krillv1.Role) (string, bool) {
	for name, role := range roleByName {
		if role == r {
			return name, true
		}
	}
	return "", false
}

// public marks procedures that need no session.
const public = krillv1.Role_ROLE_UNSPECIFIED

// policy is the minimum role for each procedure. Procedures missing from it
// are denied, and a test checks that every procedure is listed.
var policy = map[string]krillv1.Role{
	krillv1connect.HealthServiceCheckProcedure: public,

	krillv1connect.AuthServiceGetSessionProcedure:     public,
	krillv1connect.AuthServiceLoginProcedure:          public,
	krillv1connect.AuthServiceRegisterProcedure:       public,
	krillv1connect.AuthServiceLogoutProcedure:         public,
	krillv1connect.AuthServiceUpdateProfileProcedure:  krillv1.Role_ROLE_LABELER,
	krillv1connect.AuthServiceChangePasswordProcedure: krillv1.Role_ROLE_LABELER,

	krillv1connect.VideoServiceListVideosProcedure:  krillv1.Role_ROLE_LABELER,
	krillv1connect.VideoServiceGetVideoProcedure:    krillv1.Role_ROLE_LABELER,
	krillv1connect.VideoServiceCreateVideoProcedure: krillv1.Role_ROLE_DEVELOPER,
	krillv1connect.VideoServiceStartIngestProcedure: krillv1.Role_ROLE_DEVELOPER,
	krillv1connect.VideoServiceUpdateVideoProcedure: krillv1.Role_ROLE_DEVELOPER,
	krillv1connect.VideoServiceDeleteVideoProcedure: krillv1.Role_ROLE_DEVELOPER,

	krillv1connect.ClipServiceGetClipProcedure: krillv1.Role_ROLE_LABELER,

	krillv1connect.LabelServiceListLabelTypesProcedure:    krillv1.Role_ROLE_LABELER,
	krillv1connect.LabelServiceCreateLabelTypeProcedure:   krillv1.Role_ROLE_DEVELOPER,
	krillv1connect.LabelServiceUpdateLabelTypeProcedure:   krillv1.Role_ROLE_DEVELOPER,
	krillv1connect.LabelServiceDeleteLabelTypeProcedure:   krillv1.Role_ROLE_DEVELOPER,
	krillv1connect.LabelServiceReorderLabelTypesProcedure: krillv1.Role_ROLE_DEVELOPER,

	krillv1connect.AnnotationServiceCreateTrackProcedure:    krillv1.Role_ROLE_LABELER,
	krillv1connect.AnnotationServiceUpdateTrackProcedure:    krillv1.Role_ROLE_LABELER,
	krillv1connect.AnnotationServiceDeleteTrackProcedure:    krillv1.Role_ROLE_LABELER,
	krillv1connect.AnnotationServiceSetBoxProcedure:         krillv1.Role_ROLE_LABELER,
	krillv1connect.AnnotationServiceDeleteBoxProcedure:      krillv1.Role_ROLE_LABELER,
	krillv1connect.AnnotationServiceCopyBoxesProcedure:      krillv1.Role_ROLE_LABELER,
	krillv1connect.AnnotationServiceSetFrameStatusProcedure: krillv1.Role_ROLE_LABELER,

	krillv1connect.ExportServicePreviewExportProcedure: krillv1.Role_ROLE_DEVELOPER,
	krillv1connect.ExportServiceCreateExportProcedure:  krillv1.Role_ROLE_DEVELOPER,
	krillv1connect.ExportServiceListExportsProcedure:   krillv1.Role_ROLE_DEVELOPER,
	krillv1connect.ExportServiceDeleteExportProcedure:  krillv1.Role_ROLE_DEVELOPER,

	krillv1connect.UserServiceListUsersProcedure:       krillv1.Role_ROLE_ADMIN,
	krillv1connect.UserServiceUpdateUserProcedure:      krillv1.Role_ROLE_ADMIN,
	krillv1connect.UserServiceSetUserPasswordProcedure: krillv1.Role_ROLE_ADMIN,
	krillv1connect.UserServiceDeleteUserProcedure:      krillv1.Role_ROLE_ADMIN,
}
