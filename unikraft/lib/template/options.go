package template

import "strings"

func WithProjectName(projectName string) LibTemplateOption {
	return func(t *LibTemplate) {
		t.ProjectName = projectName
	}
}

func WithGitInit(gitInit bool) LibTemplateOption {
	return func(t *LibTemplate) {
		t.GitInit = gitInit
	}
}

func WithLibName(libName string) LibTemplateOption {
	return func(t *LibTemplate) {
		t.LibName = libName
	}
}

func WithLibKName(libKName string) LibTemplateOption {
	return func(t *LibTemplate) {
		t.LibKName = strings.ToLower(libKName)
		t.LibKNameUpperCase = strings.ToUpper(libKName)
	}
}

func WithVersion(version string) LibTemplateOption {
	return func(t *LibTemplate) {
		t.Version = version
	}
}

func WithDescription(description string) LibTemplateOption {
	return func(t *LibTemplate) {
		t.Description = description
	}
}

func WithAuthorName(authorName string) LibTemplateOption {
	return func(t *LibTemplate) {
		t.AuthorName = authorName
	}
}

func WithAuthorEmail(authorEmail string) LibTemplateOption {
	return func(t *LibTemplate) {
		t.AuthorEmail = authorEmail
	}
}

func WithProvideCMain(ProvideCMain bool) LibTemplateOption {
	return func(t *LibTemplate) {
		t.ProvideCMain = ProvideCMain
	}
}

func WithPatchdir(patchedir bool) LibTemplateOption {
	return func(t *LibTemplate) {
		t.WithPatchedir = patchedir
	}
}

func WithInitialBranch(initialBranch string) LibTemplateOption {
	return func(t *LibTemplate) {
		t.InitialBranch = initialBranch
	}
}

func WithCopyrightHolder(copyrightHolder string) LibTemplateOption {
	return func(t *LibTemplate) {
		t.CopyrightHolder = copyrightHolder
	}
}

func WithOriginUrl(origin string) LibTemplateOption {
	return func(t *LibTemplate) {
		t.OriginUrl = origin
	}
}
