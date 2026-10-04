import { useEffect, useState } from 'react';
import ChallengePicks from '../ChallengePicks';
import Button from '../../Button';
import Popup from '../../Popup';
import Star from '../Star';
import TextArea from '../../TextArea';

interface FinishPopupProps {
    /* Function to modify the popup state variable */
    setEnabled: React.Dispatch<React.SetStateAction<boolean>>;

    /* Judge to vote on */
    judge: Judge;

    /* State variable for determining if popup is open */
    enabled: boolean;

    /* Callback function for flagging a project */
    callback: () => Promise<void>;

    // TODO: Export all this to a global store for the judge
    /* Starred status of project */
    starred: boolean;

    /* Setter function for starred status */
    setStarred: React.Dispatch<React.SetStateAction<boolean>>;

    /* Notes for project */
    notes: string;

    /* Setter function for notes */
    setNotes: React.Dispatch<React.SetStateAction<string>>;

    /* Opt-in challenges eligible for this project and each remaining quota */
    challenges: JudgeChallengeOptions;

    /* Whether challenge eligibility is being refreshed */
    challengesLoading: boolean;

    /* Failure to load challenge eligibility */
    challengesError: string;

    /* Retry loading challenge eligibility */
    refreshChallenges: () => void;

    /* Selected challenge nominations */
    challengeStars: string[];

    /* Previous picks to replace when submitting this project. */
    replacements: Record<string, string>;
    /* Update queued replacements. */
    setReplacements: React.Dispatch<React.SetStateAction<Record<string, string>>>;

    /* Setter for selected challenge nominations */
    setChallengeStars: React.Dispatch<React.SetStateAction<string[]>>;
}

/**
 * Component to show when the user clicks the "Submit" button
 */
const FinishPopup = (props: FinishPopupProps) => {
    const [reviewChallenge, setReviewChallenge] = useState<string | null>(null);
    const [submitting, setSubmitting] = useState(false);
    useEffect(() => {
        if (!props.enabled) setReviewChallenge(null);
    }, [props.enabled]);
    if (!props.enabled) return null;

    const done = async () => {
        if (submitting) return;
        setSubmitting(true);
        try {
            await props.callback();
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <Popup
            enabled={props.enabled}
            setEnabled={props.setEnabled}
            className="text-center max-h-[90dvh] overflow-y-auto overscroll-contain"
        >
            <h1 className="text-3xl font-bold text-primary">Judge Project</h1>
            <h2 className="text-xl font-bold">Finish judging this project</h2>
            <div className="flex flex-row justify-center text-left mt-2">
                <Star
                    active={props.starred}
                    setActive={props.setStarred}
                    className="mr-4 min-h-11 min-w-11 shrink-0 inline-flex items-center justify-center"
                />
                <p className="text-light">
                    Star projects you think should win the top places in the hackathon.
                </p>
            </div>
            <div className="text-left mt-4">
                <h3 className="font-bold">Challenge Stars</h3>
                {props.challengesLoading ? (
                    <p className="text-sm text-light" role="status">
                        Loading challenge stars…
                    </p>
                ) : props.challengesError ? (
                    <div role="alert" className="text-sm text-error">
                        <p>{props.challengesError}</p>
                        <Button type="outline" onClick={props.refreshChallenges} className="mt-2">
                            Retry challenge stars
                        </Button>
                    </div>
                ) : props.challenges.challenges.length === 0 ? (
                    <p className="text-sm text-light">
                        {props.challenges.message ||
                            (props.judge.track !== ''
                                ? 'Challenge stars are available during general judging. Your stars here count for your assigned track.'
                                : 'This project has no challenges enabled for stars.')}
                    </p>
                ) : (
                    <>
                        <p className="text-sm text-light">
                            Star this project for each challenge you think it should win.
                        </p>
                        {props.challenges.challenges.map((challenge) => {
                            const remaining = props.challenges.remaining[challenge] ?? 0;
                            const selected = props.challengeStars.includes(challenge);
                            return (
                                <div key={challenge} className="py-2 text-sm">
                                    <div className="flex items-center gap-2">
                                        <Star
                                            className="min-h-11 min-w-11 shrink-0 inline-flex items-center justify-center"
                                            active={selected}
                                            ariaLabel={`Star project for ${challenge}`}
                                            disabled={submitting || (!selected && remaining === 0 && !props.replacements[challenge])}
                                            setActive={() => {
                                                if (selected) {
                                                    props.setReplacements((current) => {
                                                        const next = { ...current };
                                                        delete next[challenge];
                                                        return next;
                                                    });
                                                }
                                                props.setChallengeStars((current) =>
                                                    current.includes(challenge)
                                                        ? current.filter((name) => name !== challenge)
                                                        : [...current, challenge]
                                                );
                                            }}
                                        />
                                        <span className="min-w-0 break-words">
                                            {challenge} ({remaining} left of {props.challenges.limit})
                                        </span>
                                    </div>
                                    {props.replacements[challenge] && (
                                        <p className="text-sm text-primary">
                                            Replacement queued. Your previous star moves when you submit.
                                        </p>
                                    )}
                                    <Button
                                        type="outline"
                                        className="mt-1 min-h-11 text-sm"
                                        disabled={submitting}
                                        onClick={() => setReviewChallenge(reviewChallenge === challenge ? null : challenge)}
                                    >
                                        {reviewChallenge === challenge ? 'Close picks' : remaining === 0 ? 'Replace a pick' : 'Review picks'}
                                    </Button>
                                    {reviewChallenge === challenge && (
                                        <ChallengePicks
                                            challenge={challenge}
                                            onReplace={(name, previous) => {
                                                props.setReplacements((current) => ({ ...current, [name]: previous.id }));
                                                props.setChallengeStars((current) => current.includes(name) ? current : [...current, name]);
                                                setReviewChallenge(null);
                                            }}
                                        />
                                    )}
                                </div>
                            );
                        })}
                    </>
                )}
            </div>
            <h3 className="text-lighter text-sm text-left mt-2">Personal Notes</h3>
            <TextArea
                label="Type any personal comments here"
                value={props.notes}
                setValue={props.setNotes}
                className="mt-1"
            />
            <Button
                type="primary"
                onClick={done}
                disabled={props.challengesLoading || submitting}
                className="mt-4"
            >
                Submit
            </Button>
        </Popup>
    );
};

export default FinishPopup;
