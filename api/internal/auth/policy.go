package auth

import (
	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
)

const (
	// public procedures need no session, and signedIn ones need any account.
	public   krillv1.Permission = -1
	signedIn                    = krillv1.Permission_PERMISSION_UNSPECIFIED

	label            = krillv1.Permission_PERMISSION_LABEL
	manageVideos     = krillv1.Permission_PERMISSION_MANAGE_VIDEOS
	manageLabelTypes = krillv1.Permission_PERMISSION_MANAGE_LABEL_TYPES
	manageExports    = krillv1.Permission_PERMISSION_MANAGE_EXPORTS
	manageUsers      = krillv1.Permission_PERMISSION_MANAGE_USERS
)

// policy is the permission each procedure needs. Procedures missing from it
// are denied, and a test checks that every procedure is listed.
var policy = map[string]krillv1.Permission{
	krillv1connect.HealthServiceCheckProcedure: public,

	krillv1connect.AuthServiceGetSessionProcedure:     public,
	krillv1connect.AuthServiceLoginProcedure:          public,
	krillv1connect.AuthServiceRegisterProcedure:       public,
	krillv1connect.AuthServiceLogoutProcedure:         public,
	krillv1connect.AuthServiceUpdateProfileProcedure:  signedIn,
	krillv1connect.AuthServiceChangePasswordProcedure: signedIn,

	krillv1connect.VideoServiceListVideosProcedure:  label,
	krillv1connect.VideoServiceGetVideoProcedure:    label,
	krillv1connect.VideoServiceCreateVideoProcedure: manageVideos,
	krillv1connect.VideoServiceStartIngestProcedure: manageVideos,
	krillv1connect.VideoServiceUpdateVideoProcedure: manageVideos,
	krillv1connect.VideoServiceDeleteVideoProcedure: manageVideos,

	krillv1connect.ClipServiceGetClipProcedure: label,

	krillv1connect.QueueServiceGetQueueProcedure:      label,
	krillv1connect.QueueServiceClaimNextClipProcedure: label,

	krillv1connect.LabelServiceListLabelTypesProcedure:    label,
	krillv1connect.LabelServiceCreateLabelTypeProcedure:   manageLabelTypes,
	krillv1connect.LabelServiceUpdateLabelTypeProcedure:   manageLabelTypes,
	krillv1connect.LabelServiceDeleteLabelTypeProcedure:   manageLabelTypes,
	krillv1connect.LabelServiceReorderLabelTypesProcedure: manageLabelTypes,

	krillv1connect.AnnotationServiceCreateTrackProcedure:    label,
	krillv1connect.AnnotationServiceUpdateTrackProcedure:    label,
	krillv1connect.AnnotationServiceDeleteTrackProcedure:    label,
	krillv1connect.AnnotationServiceSetBoxProcedure:         label,
	krillv1connect.AnnotationServiceDeleteBoxProcedure:      label,
	krillv1connect.AnnotationServiceCopyBoxesProcedure:      label,
	krillv1connect.AnnotationServiceSetFrameStatusProcedure: label,

	krillv1connect.StatsServiceGetLeaderboardProcedure: label,
	krillv1connect.StatsServiceGetProfileProcedure:     label,

	krillv1connect.ExportServicePreviewExportProcedure: manageExports,
	krillv1connect.ExportServiceCreateExportProcedure:  manageExports,
	krillv1connect.ExportServiceListExportsProcedure:   manageExports,
	krillv1connect.ExportServiceDeleteExportProcedure:  manageExports,

	krillv1connect.UserServiceListUsersProcedure:       manageUsers,
	krillv1connect.UserServiceUpdateUserProcedure:      manageUsers,
	krillv1connect.UserServiceSetUserPasswordProcedure: manageUsers,
	krillv1connect.UserServiceDeleteUserProcedure:      manageUsers,
}
