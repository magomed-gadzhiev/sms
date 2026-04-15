import { Hero } from '../../components/public/Hero';
import { Stats } from '../../components/public/Stats';
import { FeaturesGrid } from '../../components/public/FeaturesGrid';
import { HowItWorks } from '../../components/public/HowItWorks';
import { Screenshot } from '../../components/public/Screenshot';
import { PricingPreview } from '../../components/public/PricingPreview';
import { FinalCta } from '../../components/public/FinalCta';

export function LandingPage() {
  return (
    <>
      <Hero />
      <Stats />
      <FeaturesGrid />
      <HowItWorks />
      <Screenshot />
      <PricingPreview />
      <FinalCta />
    </>
  );
}
