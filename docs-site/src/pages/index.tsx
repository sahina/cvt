import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from './index.module.css';

type FeatureItem = {
  title: string;
  description: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'OpenAPI Contract Validation',
    description:
      'Validate HTTP request/response interactions against OpenAPI v2/v3 specifications with comprehensive schema validation.',
  },
  {
    title: 'Consumer Contract Testing',
    description:
      'Register consumer expectations and validate that producers meet those contracts throughout the development lifecycle.',
  },
  {
    title: 'Producer Validation',
    description:
      'Ensure producer responses conform to published API specifications with detailed error reporting.',
  },
  {
    title: 'Multi-language SDKs',
    description:
      'Native SDKs for Node.js, Python, Go, and Java make integration seamless in any tech stack.',
  },
  {
    title: 'Breaking Change Detection',
    description:
      'Compare schema versions to detect breaking changes before they impact consumers.',
  },
  {
    title: 'Safe Deployment Checks',
    description:
      'CanIDeploy verification ensures schema changes won\'t break registered consumer contracts.',
  },
];

function Feature({title, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className="feature-card margin-bottom--lg">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/intro">
            Get Started
          </Link>
        </div>
      </div>
    </header>
  );
}

export default function Home(): React.JSX.Element {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={`Home`}
      description="Consumer and producer contract validation for OpenAPI specifications">
      <HomepageHeader />
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
      </main>
    </Layout>
  );
}
