/**
 * ConfidentialBadge — privacy level badge for marketplace listings.
 */

import { Badge } from '@/components/ui/badge';
import type { ConfidentialComputeInfo } from '@/types/network';

const TEE_LABELS: Record<string, string> = {
  'nvidia-blackwell': 'NVIDIA Blackwell TEE',
  'nvidia-h100': 'NVIDIA H100 Confidential',
  'intel-sgx': 'Intel SGX',
  'amd-sev': 'AMD SEV',
};

interface Props {
  info: ConfidentialComputeInfo;
}

export function ConfidentialBadge({ info }: Props) {
  if (info.privacyLevel === 'standard') return null;

  if (info.privacyLevel === 'confidential') {
    return (
      <div className="flex items-center gap-1.5">
        <Badge variant="default" className="text-[10px] bg-emerald-600 hover:bg-emerald-700">
          Confidential
        </Badge>
        {info.teeProvider !== 'none' && (
          <span className="text-[10px] text-emerald-600 font-medium">
            {TEE_LABELS[info.teeProvider] ?? info.teeProvider}
          </span>
        )}
        {info.secureEnclaveVerified && (
          <span className="text-[10px] text-emerald-500" title="TEE attestation verified">
            Verified
          </span>
        )}
      </div>
    );
  }

  return (
    <Badge variant="outline" className="text-[10px]">
      Private
    </Badge>
  );
}

export function ConfidentialDetailSection({ info }: Props) {
  if (info.privacyLevel === 'standard') return null;

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium">Confidential Computing</h3>
      <div className="grid grid-cols-2 gap-2 text-sm">
        <div>
          <p className="text-xs text-muted-foreground">Privacy Level</p>
          <p className="font-medium capitalize">{info.privacyLevel}</p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground">TEE Provider</p>
          <p className="font-medium">{TEE_LABELS[info.teeProvider] ?? info.teeProvider}</p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground">Encrypted Memory</p>
          <p className="font-medium">{info.encryptedMemory ? 'Yes' : 'No'}</p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground">Enclave Verified</p>
          <p className="font-medium">{info.secureEnclaveVerified ? 'Yes' : 'Pending'}</p>
        </div>
      </div>
      {info.attestationUrl && (
        <a
          href={info.attestationUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="text-xs text-primary hover:underline"
        >
          View TEE Attestation Report
        </a>
      )}
    </div>
  );
}
