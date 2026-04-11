use std::fmt;
use std::path::PathBuf;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ArtifactMatcher {
    NamedDir(&'static str),
    PrefixDir(&'static str),
    PathSuffix(&'static [&'static str]),
    VirtualEnv,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ArtifactDefinition {
    pub kind: ArtifactKind,
    pub matcher: ArtifactMatcher,
    pub project_markers: &'static [&'static str],
}

macro_rules! define_artifacts {
    (
        $(
            $kind:ident => {
                slug: $slug:literal,
                rules: {
                    $(
                        $matcher:expr => $project_markers:expr
                    ),+ $(,)?
                },
            }
        ),+ $(,)?
    ) => {
        #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
        pub enum ArtifactKind {
            $($kind),+
        }

        impl ArtifactKind {
            pub const fn as_str(self) -> &'static str {
                match self {
                    $(Self::$kind => $slug),+
                }
            }
        }

        pub const ARTIFACT_DEFINITIONS: &[ArtifactDefinition] = &[
            $(
                $(
                    ArtifactDefinition {
                        kind: ArtifactKind::$kind,
                        matcher: $matcher,
                        project_markers: $project_markers,
                    },
                )+
            )+
        ];
    };
}

define_artifacts! {
    PythonVenv => {
        slug: "python-venv",
        rules: {
            ArtifactMatcher::VirtualEnv => &["pyproject.toml", "requirements.txt", "setup.py", "Pipfile"]
        },
    },
    NodeModules => {
        slug: "node-modules",
        rules: {
            ArtifactMatcher::NamedDir("node_modules") => &["package.json"]
        },
    },
    RustTarget => {
        slug: "rust-target",
        rules: {
            ArtifactMatcher::NamedDir("target") => &["Cargo.toml"]
        },
    },
    SwiftBuild => {
        slug: "swift-build",
        rules: {
            ArtifactMatcher::NamedDir(".build") => &["Package.swift"]
        },
    },
    Elixir => {
        slug: "elixir",
        rules: {
            ArtifactMatcher::NamedDir("_build") => &["mix.exs"],
            ArtifactMatcher::NamedDir("deps") => &["mix.exs"]
        },
    },
    Haskell => {
        slug: "haskell",
        rules: {
            ArtifactMatcher::NamedDir(".stack-work") => &["stack.yaml", "*.cabal"],
            ArtifactMatcher::NamedDir("dist-newstyle") => &["*.cabal", "cabal.project"]
        },
    },
    Terraform => {
        slug: "terraform",
        rules: {
            ArtifactMatcher::NamedDir(".terraform") => &["*.tf", "*.tf.json", ".terraform.lock.hcl"]
        },
    },
    JavaGradle => {
        slug: "java-gradle",
        rules: {
            ArtifactMatcher::NamedDir(".gradle") => &["settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts"],
            ArtifactMatcher::NamedDir("build") => &["settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts"],
            ArtifactMatcher::NamedDir("out") => &["settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts"]
        },
    },
    JavaMaven => {
        slug: "java-maven",
        rules: {
            ArtifactMatcher::NamedDir("target") => &["pom.xml"]
        },
    },
    Cmake => {
        slug: "cmake",
        rules: {
            ArtifactMatcher::NamedDir("build") => &["CMakeLists.txt"],
            ArtifactMatcher::PrefixDir("cmake-build-") => &["CMakeLists.txt"],
            ArtifactMatcher::NamedDir("CMakeFiles") => &["CMakeLists.txt"]
        },
    },
    DartFlutter => {
        slug: "dart-flutter",
        rules: {
            ArtifactMatcher::NamedDir(".dart_tool") => &["pubspec.yaml"],
            ArtifactMatcher::NamedDir("build") => &["pubspec.yaml"]
        },
    },
    Ruby => {
        slug: "ruby",
        rules: {
            ArtifactMatcher::NamedDir(".bundle") => &["Gemfile"],
            ArtifactMatcher::PathSuffix(&["vendor", "bundle"]) => &["Gemfile"]
        },
    },
    Php => {
        slug: "php",
        rules: {
            ArtifactMatcher::NamedDir("vendor") => &["composer.json"]
        },
    },
    Zig => {
        slug: "zig",
        rules: {
            ArtifactMatcher::NamedDir("zig-out") => &["build.zig"],
            ArtifactMatcher::NamedDir(".zig-cache") => &["build.zig"]
        },
    },
    PlatformIo => {
        slug: "platformio",
        rules: {
            ArtifactMatcher::NamedDir(".pio") => &["platformio.ini"]
        },
    },
}

impl fmt::Display for ArtifactKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Artifact {
    pub kind: ArtifactKind,
    pub path: PathBuf,
    pub project_root: PathBuf,
    pub size_bytes: u64,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ScanOptions {
    pub root_path: PathBuf,
    pub max_depth: Option<usize>,
    pub include_links: bool,
}

pub fn sort_artifacts_by_size_desc(artifacts: &mut [Artifact]) {
    artifacts.sort_unstable_by(|a, b| {
        b.size_bytes.cmp(&a.size_bytes).then_with(|| a.path.cmp(&b.path))
    });
}
