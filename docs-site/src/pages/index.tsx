import {useState} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import CodeBlock from '@theme/CodeBlock';
import styles from './index.module.css';

type FeatureItem = {
  title: string;
  icon: string;
  description: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'OpenAPI Contract Validation',
    icon: '⬡',
    description:
      'Validate HTTP request/response interactions against OpenAPI v2/v3 specifications with comprehensive schema validation.',
  },
  {
    title: 'Consumer Contract Testing',
    icon: '⇄',
    description:
      'Register consumer expectations and validate that producers meet those contracts throughout the development lifecycle.',
  },
  {
    title: 'Producer Validation',
    icon: '✓',
    description:
      'Ensure producer responses conform to published API specifications with detailed error reporting.',
  },
  {
    title: 'Multi-language SDKs',
    icon: '◈',
    description:
      'Native SDKs for Node.js, Python, Go, and Java make integration seamless in any tech stack.',
  },
  {
    title: 'Breaking Change Detection',
    icon: '△',
    description:
      'Compare schema versions to detect breaking changes before they impact consumers.',
  },
  {
    title: 'Safe Deployment Checks',
    icon: '⊘',
    description:
      "CanIDeploy verification ensures schema changes won't break registered consumer contracts.",
  },
];

type SDKExample = {
  label: string;
  install: string;
  installLang: string;
  code: string;
  codeLang: string;
};

const sdkExamples: SDKExample[] = [
  {
    label: 'Node.js',
    installLang: 'bash',
    install: `npm install @sahina/cvt-sdk`,
    codeLang: 'typescript',
    code: `import { CvtClient } from '@sahina/cvt-sdk';

const client = new CvtClient('localhost:9550');

// Register your OpenAPI schema
await client.registerSchema('my-api', schema);

// Validate a request/response interaction
const result = await client.validateInteraction({
  schemaName: 'my-api',
  request: { method: 'GET', path: '/users/1' },
  response: { statusCode: 200, body: userData },
});`,
  },
  {
    label: 'Python',
    installLang: 'bash',
    install: `pip install cvt-sdk`,
    codeLang: 'python',
    code: `from cvt_sdk import CvtClient

client = CvtClient("localhost:9550")

# Register your OpenAPI schema
client.register_schema("my-api", schema)

# Validate a request/response interaction
result = client.validate_interaction(
    schema_name="my-api",
    request={"method": "GET", "path": "/users/1"},
    response={"status_code": 200, "body": user_data},
)`,
  },
  {
    label: 'Go',
    installLang: 'bash',
    install: `go get github.com/sahina/cvt/sdks/go`,
    codeLang: 'go',
    code: `import cvt "github.com/sahina/cvt/sdks/go"

client, _ := cvt.NewClient("localhost:9550")

// Register your OpenAPI schema
client.RegisterSchema("my-api", schema)

// Validate a request/response interaction
result, _ := client.ValidateInteraction(&cvt.InteractionRequest{
    SchemaName: "my-api",
    Request:    &cvt.HTTPRequest{Method: "GET", Path: "/users/1"},
    Response:   &cvt.HTTPResponse{StatusCode: 200, Body: userData},
})`,
  },
  {
    label: 'Java',
    installLang: 'xml',
    install: `<dependency>
  <groupId>io.github.sahina</groupId>
  <artifactId>cvt-sdk</artifactId>
  <version>1.0.0</version>
</dependency>`,
    codeLang: 'java',
    code: `import io.github.sahina.sdk.CvtClient;

var client = new CvtClient("localhost:9550");

// Register your OpenAPI schema
client.registerSchema("my-api", schema);

// Validate a request/response interaction
var result = client.validateInteraction(
    "my-api",
    new HttpRequest("GET", "/users/1"),
    new HttpResponse(200, userData)
);`,
  },
];

function Feature({title, icon, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className={styles.featureCard}>
        <div className={styles.featureIcon}>{icon}</div>
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={styles.heroBanner}>
      <div className="container">
        <div className={styles.heroContent}>
          <div className={styles.heroBadge}>
            <img src="/cvt/img/logo.svg" alt="CVT" width="72" height="72" />
          </div>
          <Heading as="h1" className={styles.heroTitle}>
            {siteConfig.title}
          </Heading>
          <p className={styles.heroSubtitle}>{siteConfig.tagline}</p>
          <div className={styles.heroButtons}>
            <Link
              className={clsx('button button--lg', styles.primaryButton)}
              to="/docs/intro">
              Get Started
            </Link>
            <Link
              className={clsx('button button--lg', styles.secondaryButton)}
              to="https://github.com/sahina/cvt">
              GitHub
            </Link>
          </div>
        </div>
      </div>
    </header>
  );
}

function QuickStart() {
  const [activeSDK, setActiveSDK] = useState(0);
  const sdk = sdkExamples[activeSDK];

  return (
    <section className={styles.quickStart}>
      <div className="container">
        <div className={styles.quickStartHeader}>
          <Heading as="h2">Get up and running in minutes</Heading>
          <p>Pick your language, install the SDK, and validate your first interaction.</p>
        </div>
        <div className={styles.sdkTabs}>
          {sdkExamples.map((s, i) => (
            <button
              key={s.label}
              className={clsx(styles.sdkTab, i === activeSDK && styles.sdkTabActive)}
              onClick={() => setActiveSDK(i)}>
              {s.label}
            </button>
          ))}
        </div>
        <div className="row">
          <div className={clsx('col col--5')}>
            <div className={styles.codeSection}>
              <div className={styles.codeSectionLabel}>Install</div>
              <CodeBlock language={sdk.installLang}>{sdk.install}</CodeBlock>
            </div>
          </div>
          <div className={clsx('col col--7')}>
            <div className={styles.codeSection}>
              <div className={styles.codeSectionLabel}>Validate</div>
              <CodeBlock language={sdk.codeLang}>{sdk.code}</CodeBlock>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function SDKBadges() {
  return (
    <section className={styles.sdkSection}>
      <div className="container">
        <div className={styles.sdkList}>
          {['Node.js', 'Python', 'Go', 'Java', 'Docker'].map((sdk) => (
            <span key={sdk} className={styles.sdkBadge}>{sdk}</span>
          ))}
        </div>
      </div>
    </section>
  );
}

export default function Home(): React.JSX.Element {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={`Home`}
      description="Consumer and producer contract validation for OpenAPI specifications">
      <HomepageHeader />
      <SDKBadges />
      <main>
        <section className={styles.features}>
          <div className="container">
            <div className="row">
              {FeatureList.map((props, idx) => (
                <Feature key={idx} {...props} />
              ))}
            </div>
          </div>
        </section>
        <QuickStart />
      </main>
    </Layout>
  );
}
